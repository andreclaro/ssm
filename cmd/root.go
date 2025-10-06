package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/andreclaro/ssm/internal/aws"
	"github.com/andreclaro/ssm/internal/config"
	"github.com/andreclaro/ssm/internal/service"
	"github.com/andreclaro/ssm/internal/storage"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var quickAddRegion string
var quickRemoveRegion string
var quickAddProfile string
var quickRemoveProfile string

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(ssm completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ ssm completion bash > /etc/bash_completion.d/ssm
  # macOS:
  $ ssm completion bash > /usr/local/etc/bash_completion.d/ssm

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ ssm completion zsh > "${fpath[1]}/_ssm"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ ssm completion fish | source

  # To load completions for each session, execute once:
  $ ssm completion fish > ~/.config/fish/completions/ssm.fish

PowerShell:

  PS> ssm completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> ssm completion powershell > ssm.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}
	},
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ssm [instance-name]",
	Short: "AWS SSM CLI tool for managing EC2 instances across accounts and regions",
	Long: `A CLI tool for discovering and connecting to EC2 instances using AWS Systems Manager (SSM)
across multiple AWS accounts and regions.

When called with an instance name, it will connect to that instance via Session Manager.
When called without arguments, it shows help.

Examples:
  ssm                                # Show help
  ssm my-instance-name               # Connect to instance via Session Manager
  ssm list                           # List all instances
  ssm list --region us-east-1        # List instances in us-east-1
  ssm list --profile myprofile       # List instances for myprofile
  ssm sync                           # Sync instances from AWS`,
	Args: cobra.MaximumNArgs(1),
	Run:  runConnect,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Provide completion for instance names
		if len(args) == 0 {
			return CompleteInstanceNames(toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize configuration (skip for setup command as it does it itself)
		if cmd.Name() != "setup" {
			if err := config.InitConfig(cfgFile); err != nil {
				logrus.WithError(err).Fatal("Failed to initialize configuration")
			}
		}

		// Initialize database (skip for setup command as it does it itself)
		if cmd.Name() != "setup" && storage.DB == nil {
			if err := storage.InitDB(); err != nil {
				logrus.WithError(err).Fatal("Failed to initialize database")
			}
		}

		// Handle quick add/remove flags for profiles and regions, then exit immediately
		if quickAddRegion != "" || quickRemoveRegion != "" || quickAddProfile != "" || quickRemoveProfile != "" {
			var hadError bool
			if quickAddRegion != "" {
				if err := storage.NewRegionRepository().EnableRegion(quickAddRegion); err != nil {
					logrus.WithError(err).Errorf("Failed to add region %s", quickAddRegion)
					hadError = true
				} else {
					fmt.Printf("Enabled region: %s\n", quickAddRegion)
				}
			}
			if quickRemoveRegion != "" {
				if err := storage.NewRegionRepository().DisableRegion(quickRemoveRegion); err != nil {
					logrus.WithError(err).Errorf("Failed to remove region %s", quickRemoveRegion)
					hadError = true
				} else {
					fmt.Printf("Disabled region: %s\n", quickRemoveRegion)
				}
			}
			if quickAddProfile != "" {
				if err := storage.NewProfileRepository().EnableProfile(quickAddProfile); err != nil {
					logrus.WithError(err).Errorf("Failed to add profile %s", quickAddProfile)
					hadError = true
				} else {
					fmt.Printf("Enabled profile: %s\n", quickAddProfile)
				}
			}
			if quickRemoveProfile != "" {
				if err := storage.NewProfileRepository().DisableProfile(quickRemoveProfile); err != nil {
					logrus.WithError(err).Errorf("Failed to remove profile %s", quickRemoveProfile)
					hadError = true
				} else {
					fmt.Printf("Disabled profile: %s\n", quickRemoveProfile)
				}
			}
			if hadError {
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Auto-setup on first run (skip for sync and setup commands)
		if cmd.Name() != "sync" && cmd.Name() != "setup" {
			if err := autoSetupIfFirstRun(); err != nil {
				logrus.WithError(err).Warn("Failed to auto-setup on first run")
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// autoSetupIfFirstRun automatically runs setup on first run
func autoSetupIfFirstRun() error {
	// Check if database has any instances
	repo := storage.NewInstanceRepository()
	stats, err := repo.GetStats()
	if err != nil {
		return fmt.Errorf("failed to check instance stats: %w", err)
	}

	totalInstances := stats["total"]
	if totalInstances > 0 {
		// Database already has instances, skip auto-setup
		return nil
	}

	logrus.Info("No instances found in database. Running initial setup...")

	// Run the setup process
	if err := runSetupInteractive(); err != nil {
		return fmt.Errorf("failed to run setup: %w", err)
	}

	logrus.Info("Initial setup completed successfully")
	return nil
}

// runSetupInteractive runs the interactive setup process
func runSetupInteractive() error {
	fmt.Println("Welcome to SSM CLI!")
	fmt.Println("===================")
	fmt.Println()
	fmt.Println("This appears to be your first time running SSM CLI.")
	fmt.Println("Let's set up your configuration.")
	fmt.Println()

	// Setup profiles
	if err := setupProfiles(); err != nil {
		return fmt.Errorf("failed to setup profiles: %w", err)
	}

	// Setup regions
	if err := setupRegions(); err != nil {
		return fmt.Errorf("failed to setup regions: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration complete! Running initial sync...")

	// Run initial sync
	svc, err := service.NewService()
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	ctx := context.Background()
	if err := svc.SyncInstances(ctx, nil, nil); err != nil {
		return fmt.Errorf("failed to run initial sync: %w", err)
	}

	fmt.Println("Initial sync completed successfully!")
	return nil
}

// setupProfiles interactively sets up AWS profiles
func setupProfiles() error {
	fmt.Println("Step 1: Configure AWS Profiles")
	fmt.Println("==============================")

	// Get available profiles
	availableProfiles, err := aws.GetAvailableProfiles()
	if err != nil {
		return fmt.Errorf("failed to get available profiles: %w", err)
	}

	if len(availableProfiles) == 0 {
		fmt.Println("No AWS profiles found. Please configure your AWS credentials first.")
		fmt.Println("See: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html")
		return fmt.Errorf("no AWS profiles available")
	}

	sort.Strings(availableProfiles)
	fmt.Printf("Found %d AWS profile(s): %v\n", len(availableProfiles), availableProfiles)
	fmt.Println()
	fmt.Println("You can:")
	fmt.Println("  1. Use all profiles (recommended for first-time setup)")
	fmt.Println("  2. Select specific profiles")
	fmt.Print("Choose an option (1 or 2): ")

	var choice int
	_, err = fmt.Scanf("%d", &choice)
	if err != nil {
		// Default to option 1 if input fails
		choice = 1
	}

	profileRepo := storage.NewProfileRepository()

	if choice == 2 {
		// Let user select specific profiles
		fmt.Println()
		fmt.Println("Available profiles:")
		for i, profile := range availableProfiles {
			fmt.Printf("  %d. %s\n", i+1, profile)
		}
		fmt.Println("Enter profile numbers separated by commas (e.g., 1,3,5), or 'all' for all profiles:")

		var input string
		fmt.Scanln(&input)

		if input == "all" {
			choice = 1 // Use all profiles
		} else {
			// Parse selected profiles
			selectedIndices := parseCommaSeparatedInts(input)
			var selectedProfiles []string

			for _, idx := range selectedIndices {
				if idx >= 1 && idx <= len(availableProfiles) {
					selectedProfiles = append(selectedProfiles, availableProfiles[idx-1])
				}
			}

			if len(selectedProfiles) == 0 {
				fmt.Println("No valid profiles selected. Using all profiles.")
				choice = 1
			} else {
				err = profileRepo.SetProfiles(selectedProfiles)
				if err != nil {
					return fmt.Errorf("failed to set selected profiles: %w", err)
				}
				fmt.Printf("Selected profiles: %v\n", selectedProfiles)
				return nil
			}
		}
	}

	// Use all profiles (choice 1 or default)
	err = profileRepo.SetProfiles(availableProfiles)
	if err != nil {
		return fmt.Errorf("failed to set all profiles: %w", err)
	}
	fmt.Printf("Using all available profiles: %v\n", availableProfiles)
	return nil
}

// setupRegions interactively sets up AWS regions
func setupRegions() error {
	fmt.Println()
	fmt.Println("Step 2: Configure AWS Regions")
	fmt.Println("=============================")

	// Use dynamically discovered regions (fallback handled in update-regions command)
	allRegions := aws.GetAvailableRegions()

	sort.Strings(allRegions)

	fmt.Printf("Common AWS regions: %v\n", allRegions)
	fmt.Println()
	fmt.Println("You can:")
	fmt.Println("  1. Use common regions (recommended)")
	fmt.Println("  2. Select specific regions")
	fmt.Print("Choose an option (1 or 2): ")

	var choice int
	_, err := fmt.Scanf("%d", &choice)
	if err != nil {
		// Default to option 1 if input fails
		choice = 1
	}

	regionRepo := storage.NewRegionRepository()

	if choice == 2 {
		// Let user select specific regions
		fmt.Println()
		fmt.Println("Available regions:")
		for i, region := range allRegions {
			fmt.Printf("  %d. %s\n", i+1, region)
		}
		fmt.Println("Enter region numbers separated by commas (e.g., 1,3,5), or 'all' for all regions:")

		var input string
		fmt.Scanln(&input)

		if input == "all" {
			choice = 1 // Use all regions
		} else {
			// Parse selected regions
			selectedIndices := parseCommaSeparatedInts(input)
			var selectedRegions []string

			for _, idx := range selectedIndices {
				if idx >= 1 && idx <= len(allRegions) {
					selectedRegions = append(selectedRegions, allRegions[idx-1])
				}
			}

			if len(selectedRegions) == 0 {
				fmt.Println("No valid regions selected. Using common regions.")
				choice = 1
			} else {
				// Disable all regions first
				allRegionsList := []string{
					"us-east-1", "us-east-2", "us-west-1", "us-west-2",
					"eu-west-1", "eu-central-1",
					"ap-southeast-1", "ap-southeast-2",
					"ca-central-1", "sa-east-1",
				}
				for _, region := range allRegionsList {
					if err := regionRepo.DisableRegion(region); err != nil {
						return fmt.Errorf("failed to disable region %s: %w", region, err)
					}
				}
				// Enable selected regions
				for _, region := range selectedRegions {
					if err := regionRepo.EnableRegion(region); err != nil {
						return fmt.Errorf("failed to enable region %s: %w", region, err)
					}
				}
				fmt.Printf("Selected regions: %v\n", selectedRegions)
				return nil
			}
		}
	}

	// Use common regions (choice 1 or default)
	for _, region := range allRegions {
		if err := regionRepo.EnableRegion(region); err != nil {
			return fmt.Errorf("failed to enable region %s: %w", region, err)
		}
	}
	fmt.Printf("Using common regions: %v\n", allRegions)
	return nil
}

// CompleteInstanceNames provides shell completion for instance names
func CompleteInstanceNames(toComplete string) ([]string, cobra.ShellCompDirective) {
	// Initialize database if not already done
	if storage.DB == nil {
		if err := config.InitConfig(""); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if err := storage.InitDB(); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// Query database for instance names
	var instances []storage.Instance
	query := storage.DB.Select("name").Where("name LIKE ?", toComplete+"%")
	if err := query.Find(&instances).Error; err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Extract names
	var names []string
	for _, instance := range instances {
		names = append(names, instance.Name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// parseCommaSeparatedInts parses comma-separated integers from a string
func parseCommaSeparatedInts(input string) []int {
	var result []int
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(part, "%d", &num); err == nil {
			result = append(result, num)
		}
	}
	return result
}

// runConnect handles connecting to an instance when an instance name is provided
func runConnect(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		// No arguments provided, show help
		cmd.Help()
		return
	}

	instanceName := args[0]

	// Create service
	svc, err := service.NewService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Connect to instance
	ctx := context.Background()
	if err := svc.ConnectToInstance(ctx, instanceName); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to instance: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Add completion command
	rootCmd.AddCommand(completionCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ssm/config.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().String("profile", "", "AWS profile to use")
	rootCmd.PersistentFlags().String("region", "", "AWS region to use")
	rootCmd.PersistentFlags().StringVar(&quickAddRegion, "add-region", "", "Enable a region for discovery and exit")
	rootCmd.PersistentFlags().StringVar(&quickRemoveRegion, "remove-region", "", "Disable a region for discovery and exit")
	rootCmd.PersistentFlags().StringVar(&quickAddProfile, "add-profile", "", "Enable a profile for discovery and exit")
	rootCmd.PersistentFlags().StringVar(&quickRemoveProfile, "remove-profile", "", "Disable a profile for discovery and exit")

	// Bind flags to viper
	viper.BindPFlag("aws.profile", rootCmd.PersistentFlags().Lookup("profile"))
	viper.BindPFlag("aws.region", rootCmd.PersistentFlags().Lookup("region"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".ssm" (without extension).
		viper.AddConfigPath(home + "/.ssm")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
