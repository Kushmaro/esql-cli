package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage connection profiles",
}

var profilesListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List configured connection profiles",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, defaultProfile, err := config.ListProfiles()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "No profiles configured. Run 'esql init' to create one.")
			return nil
		}

		sort.Strings(names)
		for _, name := range names {
			marker := "  "
			if name == defaultProfile {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}
		return nil
	},
}

func init() {
	profilesCmd.AddCommand(profilesListCmd)
	rootCmd.AddCommand(profilesCmd)
}
