package cmd

import (
	"fmt"
	"os"

	"github.com/andreclaro/ssm/internal/storage"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove instances with ConnectionLost state from the database",
	Long: `Remove all instances from the database where the state is 'ConnectionLost'.

This command is useful for cleaning up instances that have lost their connection
and are no longer accessible.

Examples:
  ssm clean    # Remove all instances with ConnectionLost state`,
	Run: runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) {
	// Initialize database
	if err := storage.InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}

	// Create repository
	repo := storage.NewInstanceRepository()

	// Delete instances with ConnectionLost state
	count, err := repo.DeleteByState("ConnectionLost")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clean instances: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed %d instance(s) from database\n", count)
}
