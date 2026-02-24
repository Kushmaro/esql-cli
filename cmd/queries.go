package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save <name> <query>",
	Short: "Save a named query for later use",
	Long: `Save an ES|QL query with a name so it can be re-run later with 'esql run'.

Queries are stored in ~/.esql.yaml. If a query with the same name already
exists it will be overwritten.

Examples:
  esql save error-spike 'FROM logs-* | WHERE log.level == "error" | STATS count = COUNT(*) BY service.name'
  esql save top-hosts 'FROM metrics | STATS avg_cpu = AVG(cpu) BY host | SORT avg_cpu DESC | LIMIT 10'
  esql save daily-count 'FROM {{index}} | WHERE @timestamp >= NOW() - {{window}} | STATS COUNT(*)'`,
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, query := args[0], args[1]

		if strings.ContainsAny(name, " |,()") {
			if !strings.ContainsAny(query, " |,()") {
				return fmt.Errorf("it looks like the arguments are swapped — expected: esql save <name> <query>\n  try: esql save %s '%s'", query, name)
			}
			return fmt.Errorf("query name %q contains spaces or special characters — use a simple name like 'my-query'", name)
		}

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		_, existed := cfg.Queries[name]
		cfg.Queries[name] = query

		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if existed {
			fmt.Fprintf(os.Stderr, "Query '%s' updated.\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "Query '%s' saved.\n", name)
		}
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a saved query",
	Long: `Run a previously saved ES|QL query by name.

Supports the same output flags as 'esql query'. Use --param to fill
{{placeholder}} values in parameterized queries.

Examples:
  esql run error-spike
  esql run error-spike --format json
  esql run daily-count --param index=logs-2025.* --param window=1 day`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runSavedQuery,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, err := config.ListQueryNames()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

var queriesCmd = &cobra.Command{
	Use:   "queries",
	Short: "Manage saved queries",
}

var queriesListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all saved queries",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if len(cfg.Queries) == 0 {
			fmt.Fprintln(os.Stderr, "No saved queries. Use 'esql save <name> <query>' to save one.")
			return nil
		}

		names := make([]string, 0, len(cfg.Queries))
		for name := range cfg.Queries {
			names = append(names, name)
		}
		sort.Strings(names)

		tbl := tablewriter.NewTable(os.Stdout,
			tablewriter.WithConfig(tablewriter.Config{
				Header: tw.CellConfig{
					Formatting: tw.CellFormatting{AutoFormat: tw.Off},
				},
			}),
		)
		tbl.Header([]string{"name", "query"})

		for _, name := range names {
			q := cfg.Queries[name]
			if len(q) > 80 {
				q = q[:77] + "..."
			}
			tbl.Append([]any{name, q})
		}

		tbl.Render()
		return nil
	},
}

var queriesRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a saved query",
	Args:  cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Queries[name]; !ok {
			return fmt.Errorf("unknown query %q (run 'esql queries list' to see saved queries)", name)
		}

		delete(cfg.Queries, name)

		if err := config.SaveFileConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Query '%s' removed.\n", name)
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, err := config.ListQueryNames()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	runCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv")
	runCmd.Flags().Bool("no-header", false, "suppress column headers")
	runCmd.Flags().Bool("quiet", false, "suppress all non-data output")
	runCmd.Flags().String("delimiter", "", "custom field delimiter (implies tsv)")
	runCmd.Flags().String("columns", "", "comma-separated list of columns to include")
	runCmd.Flags().Duration("timeout", 0, "per-query timeout (e.g. 60s, 5m)")
	runCmd.Flags().StringArray("param", nil, "parameter in key=value format for {{key}} placeholders")

	queriesCmd.AddCommand(queriesListCmd)
	queriesCmd.AddCommand(queriesRemoveCmd)

	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(queriesCmd)
}

func runSavedQuery(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.LoadFileConfig()
	if err != nil {
		return err
	}

	query, ok := cfg.Queries[name]
	if !ok {
		return fmt.Errorf("unknown query %q (run 'esql queries list' to see saved queries)", name)
	}

	params, _ := cmd.Flags().GetStringArray("param")
	query, err = config.ApplyParams(query, params)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString("format")
	delimiter, _ := cmd.Flags().GetString("delimiter")
	if format == "" {
		if delimiter != "" {
			format = "tsv"
		} else {
			format = formatter.DefaultFormat()
		}
	}

	noHeader, _ := cmd.Flags().GetBool("no-header")
	quiet, _ := cmd.Flags().GetBool("quiet")
	columns, _ := cmd.Flags().GetString("columns")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	var cols []string
	if columns != "" {
		for _, c := range strings.Split(columns, ",") {
			if t := strings.TrimSpace(c); t != "" {
				cols = append(cols, t)
			}
		}
	}

	clientCfg, err := config.ClientConfig()
	if err != nil {
		return err
	}
	if timeout > 0 {
		clientCfg.Timeout = timeout
	}
	c := client.New(clientCfg)

	opts := formatter.Options{
		Format:    format,
		NoHeader:  noHeader,
		Delimiter: delimiter,
		Columns:   cols,
		Quiet:     quiet,
	}

	return executeOne(c, query, opts, quiet)
}
