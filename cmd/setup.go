package cmd

import (
	"fmt"
	"os"

	"github.com/andreclaro/ssm/internal/config"
	"github.com/andreclaro/ssm/internal/storage"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run interactive setup to configure SSM CLI",
	Long: `Run the interactive setup process to configure AWS profiles and regions
for instance discovery. This command can be run at any time to reconfigure
your settings.`,
	Run: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) {
	fmt.Println("SSM CLI Setup")
	fmt.Println("=============")
	fmt.Println()

	// Initialize config first
	if err := config.InitConfig(""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	// Initialize database if not already done
	if storage.DB == nil {
		if err := storage.InitDB(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
			os.Exit(1)
		}
	}

	// Check if database has instances
	repo := storage.NewInstanceRepository()
	stats, err := repo.GetStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check instance stats: %v\n", err)
		os.Exit(1)
	}

	totalInstances := stats["total"]
	if totalInstances > 0 {
		fmt.Printf("Database already contains %d instances.\n", totalInstances)
		fmt.Println("This setup will not affect existing data, but you can modify your profile/region settings.")
		fmt.Println()
	}

	// Run the setup process
	if err := runSetupInteractive(); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Setup completed successfully!")
}
