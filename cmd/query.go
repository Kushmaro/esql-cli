package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

// Ensure time.Duration is used for the timeout flag registration.
var _ time.Duration

var queryCmd = &cobra.Command{
	Use:   "query [esql query string | -]",
	Short: "Execute an ES|QL query",
	Long: `Execute an ES|QL query against the configured Elasticsearch cluster
and display the results.

The query can be provided as an argument, read from stdin with '-',
or loaded from a file with --file.

Examples:
  esql query 'FROM logs-* | WHERE log.level == "error" | LIMIT 10'
  esql query 'FROM logs-* | STATS count = COUNT(*) BY service.name' -f json
  echo 'FROM logs-* | LIMIT 5' | esql query -
  esql query --file queries.esql --format json
  esql query 'FROM logs-* | LIMIT 100' --columns host.name,log.level
  esql query 'FROM logs-* | LIMIT 10' --delimiter '|'`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runQuery,
}

func init() {
	queryCmd.Flags().StringP("format", "f", "", "output format: table, json, ndjson, csv, tsv, parquet (default: table for TTY, tsv for pipes)")
	queryCmd.Flags().Bool("no-header", false, "suppress column headers in output")
	queryCmd.Flags().Bool("quiet", false, "suppress all non-data output (timing, row counts)")
	queryCmd.Flags().String("delimiter", "", "custom field delimiter for flat output (implies tsv format)")
	queryCmd.Flags().String("columns", "", "comma-separated list of columns to include in output")
	queryCmd.Flags().Duration("timeout", 0, "per-query timeout (e.g. 60s, 5m)")
	queryCmd.Flags().String("file", "", "read queries from file (separated by ; or blank lines)")
	queryCmd.Flags().StringArray("param", nil, "parameter in key=value format for {{key}} placeholders")
	queryCmd.Flags().StringP("output", "o", "", "write results to file instead of stdout")
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	if err := requireConnection(cmd); err != nil {
		return err
	}

	file, _ := cmd.Flags().GetString("file")
	queries, err := resolveQueries(args, file)
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

	cfg, err := config.ClientConfig()
	if err != nil {
		return err
	}
	if timeout > 0 {
		cfg.Timeout = timeout
	}
	c := client.New(cfg)

	opts := formatter.Options{
		Format:    format,
		NoHeader:  noHeader,
		Delimiter: delimiter,
		Columns:   cols,
		Quiet:     quiet,
	}

	output, _ := cmd.Flags().GetString("output")
	w, cleanup, err := resolveWriter(output)
	if err != nil {
		return err
	}
	defer cleanup()

	params, _ := cmd.Flags().GetStringArray("param")

	for i, query := range queries {
		if len(params) > 0 {
			query, err = config.ApplyParams(query, params)
			if err != nil {
				return err
			}
		}
		if err := executeOne(w, c, query, opts, quiet); err != nil {
			if len(queries) > 1 {
				fmt.Fprintf(os.Stderr, "Error in query %d: %s\n", i+1, err)
				continue
			}
			return err
		}

		if len(queries) > 1 && i < len(queries)-1 && !quiet && formatter.IsTTY() {
			fmt.Fprintln(os.Stderr)
		}
	}

	if output != "" && !quiet {
		fmt.Fprintf(os.Stderr, "Results written to %s\n", output)
	}

	return nil
}

func executeOne(w io.Writer, c *client.Client, query string, opts formatter.Options, quiet bool) error {
	resp, err := c.Query(query)
	if err != nil {
		return err
	}

	if err := formatter.Format(w, resp, opts); err != nil {
		return err
	}

	if !quiet && formatter.IsTTY() {
		fmt.Fprintf(os.Stderr, "\n%d rows in set (%dms)\n", len(resp.Values), resp.Took)
	}

	return nil
}

func resolveWriter(output string) (io.Writer, func(), error) {
	if output == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(output)
	if err != nil {
		return nil, nil, fmt.Errorf("creating output file: %w", err)
	}
	return f, func() { f.Close() }, nil
}

func resolveQueries(args []string, file string) ([]string, error) {
	if file != "" {
		return readQueriesFromFile(file)
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("provide a query argument, use '-' for stdin, or use --file")
	}

	if args[0] == "-" {
		return readQueriesFromReader(os.Stdin)
	}

	return []string{args[0]}, nil
}

func readQueriesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading query file: %w", err)
	}
	return splitQueries(string(data)), nil
}

func readQueriesFromReader(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return splitQueries(string(data)), nil
}

func splitQueries(input string) []string {
	var queries []string
	for _, part := range strings.Split(input, ";") {
		q := strings.TrimSpace(part)
		if q != "" {
			queries = append(queries, q)
		}
	}
	return queries
}
