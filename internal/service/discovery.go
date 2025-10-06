package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/andreclaro/ssm/internal/aws"
	"github.com/andreclaro/ssm/internal/config"
	"github.com/andreclaro/ssm/internal/storage"
)

// DiscoveryService handles instance discovery across AWS accounts and regions
type DiscoveryService struct {
	clientManager *aws.ClientManager
	repo          *storage.InstanceRepository
	semaphore     *semaphore.Weighted
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService() (*DiscoveryService, error) {
	if err := storage.InitDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	cfg := config.GetConfig()
	maxConcurrent := int64(cfg.AWS.MaxConcurrentSessions)

	return &DiscoveryService{
		clientManager: aws.NewClientManager(),
		repo:          storage.NewInstanceRepository(),
		semaphore:     semaphore.NewWeighted(maxConcurrent),
	}, nil
}

// DiscoverInstances discovers EC2 instances across all profiles and regions
func (ds *DiscoveryService) DiscoverInstances(ctx context.Context, profiles []string, regions []string) error {
	// If no regions specified, use enabled regions from database
	if len(regions) == 0 {
		regionRepo := storage.NewRegionRepository()
		enabledRegions, err := regionRepo.GetEnabledRegions()
		if err != nil {
			return fmt.Errorf("failed to get enabled regions: %w", err)
		}
		regions = enabledRegions
	}

	logrus.WithFields(logrus.Fields{
		"profiles": len(profiles),
		"regions":  len(regions),
	}).Info("Starting instance discovery")

	startTime := time.Now()
	var wg sync.WaitGroup
	errorChan := make(chan error, len(profiles)*len(regions))

	// Discover instances for each profile/region combination
	for _, profile := range profiles {
		for _, region := range regions {
			wg.Add(1)
			go func(profile, region string) {
				defer wg.Done()

				if err := ds.semaphore.Acquire(ctx, 1); err != nil {
					errorChan <- fmt.Errorf("failed to acquire semaphore for %s/%s: %w", profile, region, err)
					return
				}
				defer ds.semaphore.Release(1)

				if err := ds.discoverInstancesForProfileRegion(ctx, profile, region); err != nil {
					logrus.WithFields(logrus.Fields{
						"profile": profile,
						"region":  region,
					}).WithError(err).Warn("Failed to discover instances")
					errorChan <- err
				}
			}(profile, region)
		}
	}

	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	duration := time.Since(startTime)
	logrus.WithField("duration", duration).Info("Instance discovery completed")

	// Clean up stale instances
	if err := ds.cleanupStaleInstances(); err != nil {
		logrus.WithError(err).Warn("Failed to cleanup stale instances")
	}

	if len(errors) > 0 {
		return fmt.Errorf("discovery completed with %d errors", len(errors))
	}

	return nil
}

// discoverInstancesForProfileRegion discovers instances for a specific profile/region
func (ds *DiscoveryService) discoverInstancesForProfileRegion(ctx context.Context, profile, region string) error {
	logrus.WithFields(logrus.Fields{
		"profile": profile,
		"region":  region,
	}).Debug("Discovering instances")

	// Get AWS client
	client, err := ds.clientManager.GetClient(ctx, profile, region)
	if err != nil {
		return fmt.Errorf("failed to get AWS client: %w", err)
	}

	// Describe EC2 instances
	instances, err := ds.describeInstances(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to describe instances: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"profile":   profile,
		"region":    region,
		"instances": len(instances),
	}).Debug("Found instances")

	// Save EC2 instances to database in a single transaction
	batch := make([]*storage.Instance, 0, len(instances))
	for _, ec2Instance := range instances {
		batch = append(batch, storage.ConvertEC2Instance(ec2Instance, region, profile, client.AccountID))
	}
	if err := ds.repo.SaveOrUpdateBatch(batch); err != nil {
		logrus.WithFields(logrus.Fields{
			"profile": profile,
			"region":  region,
		}).WithError(err).Warn("Failed to save EC2 instances batch")
	}

	// Describe SSM managed instances and merge without duplicating EC2 instances
	managedInstances, err := ds.describeSSMManagedInstances(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to list SSM managed instances: %w", err)
	}

	// Save SSM managed instances in a batch (mi-* only)
	var ssmBatch []*storage.Instance
	for _, mi := range managedInstances {
		if mi.InstanceId == nil {
			continue
		}
		if len(*mi.InstanceId) >= 3 && (*mi.InstanceId)[:3] == "mi-" {
			ssmBatch = append(ssmBatch, storage.ConvertSSMManagedInstance(mi, region, profile, client.AccountID))
		}
	}
	if len(ssmBatch) > 0 {
		if err := ds.repo.SaveOrUpdateBatch(ssmBatch); err != nil {
			logrus.WithFields(logrus.Fields{
				"profile": profile,
				"region":  region,
			}).WithError(err).Warn("Failed to save SSM instances batch")
		}
	}

	return nil
}

// describeInstances describes EC2 instances with pagination
func (ds *DiscoveryService) describeInstances(ctx context.Context, client *aws.Client) ([]types.Instance, error) {
	input := &ec2.DescribeInstancesInput{}

	var instances []types.Instance
	paginator := ec2.NewDescribeInstancesPaginator(client.EC2Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			instances = append(instances, reservation.Instances...)
		}
	}

	return instances, nil
}

// describeSSMManagedInstances lists SSM managed instances with pagination
func (ds *DiscoveryService) describeSSMManagedInstances(ctx context.Context, client *aws.Client) ([]ssmtypes.InstanceInformation, error) {
	ssmMgr := aws.NewSSMSessionManager(client)
	instances, err := ssmMgr.ListManagedInstances(ctx)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

// cleanupStaleInstances removes instances that haven't been seen recently
func (ds *DiscoveryService) cleanupStaleInstances() error {
	cfg := config.GetConfig()
	ttlDuration, err := time.ParseDuration(cfg.Discovery.TTL)
	if err != nil {
		return fmt.Errorf("invalid TTL duration: %w", err)
	}

	return ds.repo.DeleteStale(ttlDuration)
}

// GetStats returns discovery statistics
func (ds *DiscoveryService) GetStats() (map[string]int, error) {
	return ds.repo.GetStats()
}
