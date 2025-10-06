package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/andreclaro/ssm/internal/service"
	"github.com/spf13/cobra"
)

var (
	syncProfile string
	syncRegion  string
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize instances from AWS",
	Long: `Synchronize EC2 instances from AWS across all configured profiles and regions.

This command will discover all EC2 instances and update the local database.

Examples:
  ssm sync                          # Sync all instances
  ssm sync --profile myprofile      # Sync instances for myprofile only
  ssm sync --region us-east-1       # Sync instances in us-east-1 only
  ssm sync --profile dev --region us-west-2  # Sync specific profile and region`,
	Run: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringVar(&syncProfile, "profile", "", "Sync only specified AWS profile")
	syncCmd.Flags().StringVar(&syncRegion, "region", "", "Sync only specified AWS region")
}

func runSync(cmd *cobra.Command, args []string) {
	// Create service
	svc, err := service.NewService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Prepare filters
	var profile, region *string
	if syncProfile != "" {
		profile = &syncProfile
	}
	if syncRegion != "" {
		region = &syncRegion
	}

	// Sync instances
	ctx := context.Background()
	if err := svc.SyncInstances(ctx, profile, region); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync instances: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Instance synchronization completed successfully")
}
