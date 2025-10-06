package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andreclaro/ssm/internal/aws"
	"github.com/andreclaro/ssm/internal/storage"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// allRegions will be dynamically populated on command run
var allRegions []string

// updateRegionsCmd represents the update-regions command
var updateRegionsCmd = &cobra.Command{
	Use:   "update-regions",
	Short: "Update the list of AWS regions to monitor",
	Long: `Interactive command to select which AWS regions to include in instance discovery.

This command allows you to enable or disable specific AWS regions for instance discovery.
Only enabled regions will be scanned for EC2 instances.`,
	Run: runUpdateRegions,
}

func init() {
	rootCmd.AddCommand(updateRegionsCmd)
}

func runUpdateRegions(cmd *cobra.Command, args []string) {
	// Dynamically load regions (fall back to static)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	regionsDyn, err := aws.GetAvailableRegionsDynamic(ctx, "")
	if err != nil || len(regionsDyn) == 0 {
		logrus.WithError(err).Warn("Falling back to static region list")
		regionsDyn = aws.GetAvailableRegions()
	}
	allRegions = regionsDyn

	// Get current regions status
	regionRepo := storage.NewRegionRepository()
	regions, err := regionRepo.GetAllRegions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get regions: %v\n", err)
		os.Exit(1)
	}

	// Create a map for quick lookup
	regionMap := make(map[string]storage.Region)
	for _, region := range regions {
		regionMap[region.Region] = region
	}

	fmt.Println("AWS Regions Configuration")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Select regions to enable for instance discovery:")
	fmt.Println("Enter the numbers of regions to toggle (comma-separated), or 'all' to enable all, 'none' to disable all:")
	fmt.Println()

	// Display regions with status
	// Sort regions alphabetically for display
	sorted := make([]string, len(allRegions))
	copy(sorted, allRegions)
	sort.Strings(sorted)
	for i, regionName := range sorted {
		status := "[ ]"
		if region, exists := regionMap[regionName]; exists && region.Enabled {
			status = "[✓]"
		}
		fmt.Printf("%2d. %s %s\n", i+1, status, regionName)
	}

	fmt.Println()
	fmt.Print("Enter your choice: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
		os.Exit(1)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	if input == "all" {
		// Enable all regions
		for _, regionName := range allRegions {
			if err := regionRepo.EnableRegion(regionName); err != nil {
				logrus.WithError(err).WithField("region", regionName).Warn("Failed to enable region")
			}
		}
		fmt.Println("All regions enabled")

	} else if input == "none" {
		// Disable all regions
		for _, regionName := range allRegions {
			if err := regionRepo.DisableRegion(regionName); err != nil {
				logrus.WithError(err).WithField("region", regionName).Warn("Failed to disable region")
			}
		}
		fmt.Println("All regions disabled")

	} else {
		// Parse comma-separated numbers
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			index, err := strconv.Atoi(part)
			if err != nil || index < 1 || index > len(allRegions) {
				fmt.Printf("Invalid selection: %s (must be 1-%d)\n", part, len(allRegions))
				continue
			}

			// Map index to the alphabetically sorted list for consistent selection
			sorted := make([]string, len(allRegions))
			copy(sorted, allRegions)
			sort.Strings(sorted)
			regionName := sorted[index-1]
			region, exists := regionMap[regionName]

			if exists && region.Enabled {
				// Disable region
				if err := regionRepo.DisableRegion(regionName); err != nil {
					logrus.WithError(err).WithField("region", regionName).Warn("Failed to disable region")
				} else {
					fmt.Printf("Disabled region: %s\n", regionName)
				}
			} else {
				// Enable region
				if err := regionRepo.EnableRegion(regionName); err != nil {
					logrus.WithError(err).WithField("region", regionName).Warn("Failed to enable region")
				} else {
					fmt.Printf("Enabled region: %s\n", regionName)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("Region configuration updated. Run 'ssm sync' to discover instances in the selected regions.")
}
