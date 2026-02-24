package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

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

var profilesSetCmd = &cobra.Command{
	Use:          "set <profile>",
	Short:        "Set the default connection profile",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("unknown profile %q (run 'esql profiles list' to see available profiles)", name)
		}

		cfg.DefaultProfile = name
		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Default profile set to '%s'\n", name)
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, _, err := config.ListProfiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

var profilesRemoveCmd = &cobra.Command{
	Use:          "remove <profile>",
	Short:        "Remove a connection profile",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("unknown profile %q (run 'esql profiles list' to see available profiles)", name)
		}

		fmt.Fprintf(os.Stderr, "Remove profile '%s'? [y/N]: ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}

		delete(cfg.Profiles, name)

		if cfg.DefaultProfile == name {
			cfg.DefaultProfile = ""
			for remaining := range cfg.Profiles {
				cfg.DefaultProfile = remaining
				break
			}
		}

		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Profile '%s' removed.\n", name)
		if cfg.DefaultProfile != "" {
			fmt.Fprintf(os.Stderr, "Default profile is now '%s'.\n", cfg.DefaultProfile)
		}
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, _, err := config.ListProfiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesSetCmd)
	profilesCmd.AddCommand(profilesRemoveCmd)
	rootCmd.AddCommand(profilesCmd)
}
