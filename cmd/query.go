package cmd

import (
	"fmt"
	"os"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [esql query string]",
	Short: "Execute an ES|QL query",
	Long: `Execute an ES|QL query against the configured Elasticsearch cluster
and display the results.

Examples:
  esql query 'FROM logs-* | WHERE log.level == "error" | LIMIT 10'
  esql query 'FROM logs-* | STATS count = COUNT(*) BY service.name' -f json
  esql query 'FROM logs-* | KEEP message | LIMIT 50' -f csv --no-header`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runQuery,
}

func init() {
	queryCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv (default: table for TTY, tsv for pipes)")
	queryCmd.Flags().Bool("no-header", false, "suppress column headers in output")
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	query := args[0]

	format, _ := cmd.Flags().GetString("format")
	if format == "" {
		format = formatter.DefaultFormat()
	}

	noHeader, _ := cmd.Flags().GetBool("no-header")

	cfg := config.ClientConfig()
	c := client.New(cfg)

	resp, err := c.Query(query)
	if err != nil {
		return err
	}

	opts := formatter.Options{
		Format:   format,
		NoHeader: noHeader,
	}

	if err := formatter.Format(os.Stdout, resp, opts); err != nil {
		return err
	}

	if formatter.IsTTY() {
		fmt.Fprintf(os.Stderr, "\n%d rows in set (%dms)\n", len(resp.Values), resp.Took)
	}

	return nil
}
