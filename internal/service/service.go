package service

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/andreclaro/ssm/internal/aws"
	"github.com/andreclaro/ssm/internal/storage"
)

// Service represents the main SSM CLI service
type Service struct {
	discovery *DiscoveryService
}

// NewService creates a new service instance
func NewService() (*Service, error) {
	discovery, err := NewDiscoveryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery service: %w", err)
	}

	return &Service{
		discovery: discovery,
	}, nil
}

// SyncInstances synchronizes instances across all configured profiles and regions
func (s *Service) SyncInstances(ctx context.Context, profile, region *string) error {
	logrus.Info("Starting instance synchronization")

	// Get available profiles
	var profiles []string
	if profile != nil {
		profiles = []string{*profile}
	} else {
		profileRepo := storage.NewProfileRepository()
		var err error
		profiles, err = profileRepo.GetEnabledProfiles()
		if err != nil {
			return fmt.Errorf("failed to get enabled profiles: %w", err)
		}
	}

	// Get available regions
	var regions []string
	if region != nil {
		regions = []string{*region}
	}
	// If region is nil, pass empty slice to let discovery service use enabled regions

	// Discover instances
	if err := s.discovery.DiscoverInstances(ctx, profiles, regions); err != nil {
		return fmt.Errorf("failed to discover instances: %w", err)
	}

	logrus.Info("Instance synchronization completed")
	return nil
}

// ListInstances lists instances with optional filters
func (s *Service) ListInstances(profile, region *string) ([]storage.Instance, error) {
	repo := storage.NewInstanceRepository()
	filter := &storage.InstanceFilter{
		Profile: profile,
		Region:  region,
	}

	instances, err := repo.List(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	return instances, nil
}

// ConnectToInstance connects to an instance via SSM Session Manager
func (s *Service) ConnectToInstance(ctx context.Context, instanceName string) error {
	repo := storage.NewInstanceRepository()

	// Find instance by name
	instance, err := repo.FindByName(instanceName)
	if err != nil {
		return fmt.Errorf("failed to find instance: %w", err)
	}

	if instance == nil {
		return fmt.Errorf("instance '%s' not found", instanceName)
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": instance.InstanceID,
		"name":        instance.Name,
		"profile":     instance.Profile,
		"region":      instance.Region,
	}).Info("Connecting to instance")

	// Get AWS client
	clientManager := aws.NewClientManager()
	client, err := clientManager.GetClient(ctx, instance.Profile, instance.Region)
	if err != nil {
		return fmt.Errorf("failed to get AWS client: %w", err)
	}

	// Start SSM session
	ssmManager := aws.NewSSMSessionManager(client)
	if err := ssmManager.StartSession(ctx, instance.InstanceID); err != nil {
		return fmt.Errorf("failed to start SSM session: %w", err)
	}

	return nil
}

// PortForwardToInstance starts an SSM port forwarding session to the given instance name
func (s *Service) PortForwardToInstance(ctx context.Context, instanceName string, localPort, remotePort int) error {
	repo := storage.NewInstanceRepository()

	// Find instance by name
	instance, err := repo.FindByName(instanceName)
	if err != nil {
		return fmt.Errorf("failed to find instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance '%s' not found", instanceName)
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": instance.InstanceID,
		"name":        instance.Name,
		"profile":     instance.Profile,
		"region":      instance.Region,
		"local_port":  localPort,
		"remote_port": remotePort,
	}).Info("Starting port forwarding to instance")

	// Get AWS client
	clientManager := aws.NewClientManager()
	client, err := clientManager.GetClient(ctx, instance.Profile, instance.Region)
	if err != nil {
		return fmt.Errorf("failed to get AWS client: %w", err)
	}

	// Start SSM port forwarding session
	ssmManager := aws.NewSSMSessionManager(client)
	if err := ssmManager.StartPortForwarding(ctx, instance.InstanceID, localPort, remotePort); err != nil {
		return fmt.Errorf("failed to start SSM port forwarding: %w", err)
	}
	return nil
}

// PortMapping represents a local to remote port mapping
type PortMapping struct {
	LocalPort  int
	RemotePort int
}

// PortForwardToInstanceMultiple starts multiple concurrent SSM port forwarding sessions
func (s *Service) PortForwardToInstanceMultiple(ctx context.Context, instanceName string, mappings []PortMapping) error {
	if len(mappings) == 0 {
		return fmt.Errorf("no port mappings provided")
	}

	repo := storage.NewInstanceRepository()
	instance, err := repo.FindByName(instanceName)
	if err != nil {
		return fmt.Errorf("failed to find instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance '%s' not found", instanceName)
	}

	clientManager := aws.NewClientManager()
	client, err := clientManager.GetClient(ctx, instance.Profile, instance.Region)
	if err != nil {
		return fmt.Errorf("failed to get AWS client: %w", err)
	}

	ssmManager := aws.NewSSMSessionManager(client)

	// Start each mapping in its own goroutine and wait; if any fails, return the error
	errCh := make(chan error, len(mappings))
	for _, m := range mappings {
		m := m
		go func() {
			errCh <- ssmManager.StartPortForwarding(ctx, instance.InstanceID, m.LocalPort, m.RemotePort)
		}()
	}

	// If any of them errors immediately, return it; otherwise block forever until user exits sessions
	// Collect first error if any
	for i := 0; i < len(mappings); i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

// GetStats returns service statistics
func (s *Service) GetStats() (map[string]int, error) {
	return s.discovery.GetStats()
}

// ValidateProfiles validates that the specified profiles have valid credentials
func (s *Service) ValidateProfiles(ctx context.Context, profiles []string) error {
	clientManager := aws.NewClientManager()

	for _, profile := range profiles {
		if err := clientManager.ValidateCredentials(ctx, profile); err != nil {
			logrus.WithField("profile", profile).WithError(err).Warn("Profile validation failed")
			return fmt.Errorf("invalid credentials for profile %s: %w", profile, err)
		}
	}

	return nil
}
