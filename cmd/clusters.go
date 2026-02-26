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

var clustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "Manage cluster connections",
}

var clustersListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List configured cluster connections",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, defaultCluster, err := config.ListClusters()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "No clusters configured. Run 'esql init' to create one.")
			return nil
		}

		sort.Strings(names)
		for _, name := range names {
			marker := "  "
			if name == defaultCluster {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}
		return nil
	},
}

var clustersSetCmd = &cobra.Command{
	Use:          "set <cluster>",
	Short:        "Set the default cluster connection",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Clusters[name]; !ok {
			return fmt.Errorf("unknown cluster %q (run 'esql clusters list' to see available clusters)", name)
		}

		cfg.DefaultCluster = name
		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Default cluster set to '%s'\n", name)
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, _, err := config.ListClusters()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

var clustersRemoveCmd = &cobra.Command{
	Use:          "remove <cluster>",
	Short:        "Remove a cluster connection",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Clusters[name]; !ok {
			return fmt.Errorf("unknown cluster %q (run 'esql clusters list' to see available clusters)", name)
		}

		fmt.Fprintf(os.Stderr, "Remove cluster '%s'? [y/N]: ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}

		delete(cfg.Clusters, name)

		if cfg.DefaultCluster == name {
			cfg.DefaultCluster = ""
			for remaining := range cfg.Clusters {
				cfg.DefaultCluster = remaining
				break
			}
		}

		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Cluster '%s' removed.\n", name)
		if cfg.DefaultCluster != "" {
			fmt.Fprintf(os.Stderr, "Default cluster is now '%s'.\n", cfg.DefaultCluster)
		}
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, _, err := config.ListClusters()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	clustersCmd.AddCommand(clustersListCmd)
	clustersCmd.AddCommand(clustersSetCmd)
	clustersCmd.AddCommand(clustersRemoveCmd)
	rootCmd.AddCommand(clustersCmd)
}
