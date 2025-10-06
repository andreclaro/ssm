package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

// Client represents AWS service clients for a specific profile
type Client struct {
	Profile   string
	Region    string
	AccountID string
	Config    aws.Config

	// Service clients
	EC2Client *ec2.Client
	SSMClient *ssm.Client
	STSClient *sts.Client
}

// ClientManager manages AWS clients for different profiles and regions
type ClientManager struct {
	clients map[string]*Client // key: profile:region
	mutex   sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

// GetClient returns an AWS client for the specified profile and region
func (cm *ClientManager) GetClient(ctx context.Context, profile, region string) (*Client, error) {
	key := profile + ":" + region

	// Check cache first
	cm.mutex.RLock()
	if client, exists := cm.clients[key]; exists {
		cm.mutex.RUnlock()
		return client, nil
	}
	cm.mutex.RUnlock()

	// Create new client
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Double-check after acquiring write lock
	if client, exists := cm.clients[key]; exists {
		return client, nil
	}

	client, err := cm.createClient(ctx, profile, region)
	if err != nil {
		return nil, err
	}

	cm.clients[key] = client
	logrus.WithFields(logrus.Fields{
		"profile":    profile,
		"region":     region,
		"account_id": client.AccountID,
	}).Debug("Created AWS client")

	return client, nil
}

// createClient creates a new AWS client for the specified profile and region
func (cm *ClientManager) createClient(ctx context.Context, profile, region string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for profile %s: %w", profile, err)
	}

	// Create service clients
	ec2Client := ec2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)
	stsClient := sts.NewFromConfig(cfg)

	// Get account ID
	accountID, err := cm.getAccountID(ctx, stsClient)
	if err != nil {
		logrus.WithError(err).WithField("profile", profile).Warn("Failed to get account ID")
		accountID = "unknown"
	}

	client := &Client{
		Profile:   profile,
		Region:    region,
		AccountID: accountID,
		Config:    cfg,
		EC2Client: ec2Client,
		SSMClient: ssmClient,
		STSClient: stsClient,
	}

	return client, nil
}

// getAccountID retrieves the AWS account ID using STS
func (cm *ClientManager) getAccountID(ctx context.Context, stsClient *sts.Client) (string, error) {
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}

	if result.Account == nil {
		return "", fmt.Errorf("account ID is nil")
	}

	return *result.Account, nil
}

// GetAvailableProfiles returns a list of available AWS profiles
func GetAvailableProfiles() ([]string, error) {
	profileSet := make(map[string]bool)

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{"default"}, nil // fallback
	}

	// Read AWS config file (~/.aws/config)
	configPath := filepath.Join(homeDir, ".aws", "config")
	if profiles, err := readProfilesFromFile(configPath, true); err == nil {
		for _, profile := range profiles {
			profileSet[profile] = true
		}
	}

	// Read AWS credentials file (~/.aws/credentials)
	credentialsPath := filepath.Join(homeDir, ".aws", "credentials")
	if profiles, err := readProfilesFromFile(credentialsPath, false); err == nil {
		for _, profile := range profiles {
			profileSet[profile] = true
		}
	}

	// If no profiles found, return default
	if len(profileSet) == 0 {
		return []string{"default"}, nil
	}

	// Convert map to slice
	profiles := make([]string, 0, len(profileSet))
	for profile := range profileSet {
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// readProfilesFromFile reads AWS profiles from a config file
func readProfilesFromFile(filePath string, isConfigFile bool) ([]string, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, err
	}

	// Parse the INI file
	cfg, err := ini.Load(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AWS config file %s: %w", filePath, err)
	}

	var profiles []string

	// Get all section names
	sectionNames := cfg.SectionStrings()

	// Iterate through all sections
	for _, sectionName := range sectionNames {
		var profileName string

		if isConfigFile {
			// In config file, sections are named "profile name" or just "name"
			if strings.HasPrefix(sectionName, "profile ") {
				profileName = strings.TrimPrefix(sectionName, "profile ")
			} else if sectionName != "default" {
				// Skip non-profile sections in config file
				continue
			} else {
				profileName = sectionName
			}
		} else {
			// In credentials file, sections are profile names directly
			profileName = sectionName
		}

		if profileName != "" {
			profiles = append(profiles, profileName)
		}
	}

	return profiles, nil
}

// GetAvailableRegions returns a list of available AWS regions
func GetAvailableRegions() []string {
	// AWS regions as of 2024
	return []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-central-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
		"ca-central-1", "sa-east-1",
	}
}

// GetAvailableRegionsDynamic fetches regions using DescribeRegions. Falls back to static list on error.
func GetAvailableRegionsDynamic(ctx context.Context, profile string) ([]string, error) {
	// Load config with any region (AWS requires a region but DescribeRegions works from any known region)
	// Prefer us-east-1 as it is ubiquitous
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for regions: %w", err)
	}

	cli := ec2.NewFromConfig(cfg)
	out, err := cli.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
		Filters:    []ec2types.Filter{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe regions: %w", err)
	}

	regions := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		if r.RegionName != nil {
			regions = append(regions, *r.RegionName)
		}
	}
	sort.Strings(regions)
	return regions, nil
}

// ValidateCredentials validates that the profile has valid credentials
func (cm *ClientManager) ValidateCredentials(ctx context.Context, profile string) error {
	// Try to create a client for us-east-1 (arbitrary region)
	client, err := cm.GetClient(ctx, profile, "us-east-1")
	if err != nil {
		return fmt.Errorf("failed to create client for profile %s: %w", profile, err)
	}

	// Try to get caller identity to validate credentials
	_, err = cm.getAccountID(ctx, client.STSClient)
	if err != nil {
		return fmt.Errorf("invalid credentials for profile %s: %w", profile, err)
	}

	return nil
}
