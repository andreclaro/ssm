package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/andreclaro/ssm/internal/service"
	"github.com/spf13/cobra"
)

var (
	listProfile string
	listRegion  string
	listAll     bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered EC2 instances",
	Long: `List all discovered EC2 instances across AWS accounts and regions.

Examples:
  ssm list                              # List all instances
  ssm list --profile myprofile          # List instances for myprofile
  ssm list --region us-east-1           # List instances in us-east-1
  ssm list --profile dev --region us-west-2  # List instances for dev profile in us-west-2`,
	Run: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&listProfile, "profile", "", "Filter by AWS profile")
	listCmd.Flags().StringVar(&listRegion, "region", "", "Filter by AWS region")
	listCmd.Flags().BoolVar(&listAll, "all", false, "Show all columns")
}

func runList(cmd *cobra.Command, args []string) {
	// Create service
	svc, err := service.NewService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Prepare filters
	var profile, region *string
	if listProfile != "" {
		profile = &listProfile
	}
	if listRegion != "" {
		region = &listRegion
	}

	// List instances
	instances, err := svc.ListInstances(profile, region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list instances: %v\n", err)
		os.Exit(1)
	}

	// When not --all, only show SSM managed instances that are Online
	if !listAll {
		j := 0
		for _, inst := range instances {
			if strings.HasPrefix(inst.InstanceID, "mi-") && strings.EqualFold(inst.State, "Online") {
				instances[j] = inst
				j++
			}
		}
		instances = instances[:j]
	}

	// Display results
	if len(instances) == 0 {
		fmt.Println("No instances found")
		return
	}

	// Create tab writer for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	if listAll {
		fmt.Fprintln(w, "NAME\tINSTANCE ID\tREGION\tPROFILE\tACCOUNT ID\tSTATE\tPLATFORM")
	} else {
		fmt.Fprintln(w, "NAME\tREGION\tPROFILE")
	}

	// Print instances
	for _, instance := range instances {
		name := instance.Name
		if name == "" {
			name = instance.InstanceID
		}

		if listAll {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				name,
				instance.InstanceID,
				instance.Region,
				instance.Profile,
				instance.AccountID,
				instance.State,
				instance.Platform,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				name,
				instance.Region,
				instance.Profile,
			)
		}
	}
}
