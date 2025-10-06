package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/sirupsen/logrus"
)

// SSMSessionManager handles SSM Session Manager operations
type SSMSessionManager struct {
	client *Client
}

// NewSSMSessionManager creates a new SSM session manager
func NewSSMSessionManager(client *Client) *SSMSessionManager {
	return &SSMSessionManager{
		client: client,
	}
}

// StartSession starts an SSM session with the specified instance
func (sm *SSMSessionManager) StartSession(ctx context.Context, instanceID string) error {
	logrus.WithFields(logrus.Fields{
		"instance_id": instanceID,
		"profile":     sm.client.Profile,
		"region":      sm.client.Region,
	}).Info("Starting SSM session")

	// Check if instance is reachable via SSM
	if err := sm.checkInstanceReachability(ctx, instanceID); err != nil {
		return fmt.Errorf("instance not reachable via SSM: %w", err)
	}

	// Start SSM session using AWS CLI
	return sm.startSessionWithCLI(instanceID)
}

// checkInstanceReachability checks if the instance is reachable via SSM
func (sm *SSMSessionManager) checkInstanceReachability(ctx context.Context, instanceID string) error {
	input := &ssm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{
				Key:    aws.String(string(types.InstanceInformationFilterKeyInstanceIds)),
				Values: []string{instanceID},
			},
		},
	}

	result, err := sm.client.SSMClient.DescribeInstanceInformation(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe instance information: %w", err)
	}

	if len(result.InstanceInformationList) == 0 {
		return fmt.Errorf("instance not found in SSM inventory")
	}

	info := result.InstanceInformationList[0]
	if info.PingStatus == "Offline" {
		return fmt.Errorf("instance is offline (ping status: %s)", info.PingStatus)
	}

	logrus.WithField("ping_status", info.PingStatus).Debug("Instance is reachable via SSM")
	return nil
}

// startSessionWithCLI starts an SSM session using the AWS CLI
func (sm *SSMSessionManager) startSessionWithCLI(instanceID string) error {
	// Prepare AWS CLI command
	args := []string{
		"ssm", "start-session",
		"--target", instanceID,
		"--profile", sm.client.Profile,
		"--region", sm.client.Region,
	}

	// Execute AWS CLI command
	cmd := exec.Command("aws", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	logrus.WithFields(logrus.Fields{
		"command": "aws " + fmt.Sprintf("%v", args),
	}).Debug("Executing AWS CLI command")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start SSM session: %w", err)
	}

	return nil
}

// GetInstanceInformation gets detailed information about an instance from SSM
func (sm *SSMSessionManager) GetInstanceInformation(ctx context.Context, instanceID string) (*types.InstanceInformation, error) {
	input := &ssm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{
				Key:    aws.String(string(types.InstanceInformationFilterKeyInstanceIds)),
				Values: []string{instanceID},
			},
		},
	}

	result, err := sm.client.SSMClient.DescribeInstanceInformation(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance information: %w", err)
	}

	if len(result.InstanceInformationList) == 0 {
		return nil, fmt.Errorf("instance not found in SSM inventory")
	}

	return &result.InstanceInformationList[0], nil
}

// ListManagedInstances lists all instances managed by SSM
func (sm *SSMSessionManager) ListManagedInstances(ctx context.Context) ([]types.InstanceInformation, error) {
	input := &ssm.DescribeInstanceInformationInput{}

	var instances []types.InstanceInformation
	paginator := ssm.NewDescribeInstanceInformationPaginator(sm.client.SSMClient, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list managed instances: %w", err)
		}

		instances = append(instances, page.InstanceInformationList...)
	}

	return instances, nil
}
