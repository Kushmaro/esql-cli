package cmd

import (
	"os"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for esql.

To load completions:

Bash:
  $ source <(esql completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ esql completion bash > /etc/bash_completion.d/esql
  # macOS:
  $ esql completion bash > $(brew --prefix)/etc/bash_completion.d/esql

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  $ source <(esql completion zsh)

  # To load completions for each session, execute once:
  $ esql completion zsh > "${fpath[1]}/_esql"

Fish:
  $ esql completion fish | source

  # To load completions for each session, execute once:
  $ esql completion fish > ~/.config/fish/completions/esql.fish

PowerShell:
  PS> esql completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add the output to your profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	registerDynamicCompletions()
}

func registerDynamicCompletions() {
	formatValues := []string{"table\tformatted table output", "json\tJSON output", "csv\tcomma-separated values", "tsv\ttab-separated values"}

	rootCmd.RegisterFlagCompletionFunc("cluster", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names, _, err := config.ListClusters()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	queryCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return formatValues, cobra.ShellCompDirectiveNoFileComp
	})

	dataCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return formatValues, cobra.ShellCompDirectiveNoFileComp
	})

	fieldsCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return formatValues, cobra.ShellCompDirectiveNoFileComp
	})

	fieldsCmd.ValidArgsFunction = completeDataSourceNames

	dataCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeDataSourceNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return []string{"fields"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeDataSourceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg, err := config.ClientConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg.Timeout = 5_000_000_000 // 5s for completion
	c := client.New(cfg)

	seen := make(map[string]bool)
	var names []string

	indices, err := c.ListIndices("")
	if err == nil {
		for _, idx := range indices {
			if !strings.HasPrefix(idx.Index, ".") && !seen[idx.Index] {
				seen[idx.Index] = true
				names = append(names, idx.Index)
			}
		}
	}

	streams, err := c.ListDataStreams("")
	if err == nil {
		for _, ds := range streams {
			if !seen[ds.Name] {
				seen[ds.Name] = true
				names = append(names, ds.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
