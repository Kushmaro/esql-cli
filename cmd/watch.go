package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch <query>",
	Short: "Re-run a query on an interval",
	Long: `Re-run an ES|QL query at a fixed interval, refreshing the terminal
each time. Useful for live monitoring of metrics or logs.

Press Ctrl-C to stop.

Examples:
  esql watch 'FROM metrics | STATS avg_cpu = AVG(cpu) BY host' --every 5s
  esql watch 'FROM logs-* | STATS count = COUNT(*) BY log.level' --every 10s --format json
  esql watch --saved error-spike --every 30s
  esql watch 'FROM {{index}} | STATS COUNT(*)' --every 5s --param index=logs-*`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runWatch,
}

func init() {
	watchCmd.Flags().Duration("every", 5*time.Second, "refresh interval (e.g. 2s, 1m)")
	watchCmd.Flags().StringP("format", "f", "table", "output format: table, json, csv, tsv")
	watchCmd.Flags().Bool("no-header", false, "suppress column headers")
	watchCmd.Flags().String("delimiter", "", "custom field delimiter (implies tsv)")
	watchCmd.Flags().String("columns", "", "comma-separated list of columns to include")
	watchCmd.Flags().Duration("timeout", 0, "per-query timeout (e.g. 60s, 5m)")
	watchCmd.Flags().StringArray("param", nil, "parameter in key=value format for {{key}} placeholders")
	watchCmd.Flags().String("saved", "", "run a saved query by name instead of an inline query")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	savedName, _ := cmd.Flags().GetString("saved")
	var query string

	if savedName != "" {
		cfg, err := config.LoadFileConfig()
		if err != nil {
			return err
		}
		q, ok := cfg.Queries[savedName]
		if !ok {
			return fmt.Errorf("unknown query %q (run 'esql queries list' to see saved queries)", savedName)
		}
		query = q
	} else if len(args) == 1 {
		query = args[0]
	} else {
		return fmt.Errorf("provide a query argument or use --saved <name>")
	}

	params, _ := cmd.Flags().GetStringArray("param")
	query, err := config.ApplyParams(query, params)
	if err != nil {
		return err
	}

	every, _ := cmd.Flags().GetDuration("every")
	format, _ := cmd.Flags().GetString("format")
	delimiter, _ := cmd.Flags().GetString("delimiter")
	if delimiter != "" && format == "table" {
		format = "tsv"
	}
	noHeader, _ := cmd.Flags().GetBool("no-header")
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
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	// Run immediately on first iteration
	clearAndRun(c, query, opts, every)

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\nStopped.")
			return nil
		case <-ticker.C:
			clearAndRun(c, query, opts, every)
		}
	}
}

func clearAndRun(c *client.Client, query string, opts formatter.Options, every time.Duration) {
	fmt.Fprint(os.Stdout, "\033[2J\033[H")

	now := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "Every %s — %s\n\n", every, now)

	resp, err := c.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		if hint := client.ErrorHint(err); hint != "" {
			fmt.Fprintf(os.Stderr, "  Hint: %s\n", hint)
		}
		return
	}

	if err := formatter.Format(os.Stdout, resp, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%d rows (%dms)\n", len(resp.Values), resp.Took)
}
