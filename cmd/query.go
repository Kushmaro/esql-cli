package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

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
	addNativeQueryFlags(queryCmd)
	queryCmd.Flags().Bool("raw", false, "return the full JSON response including metadata")
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

	opts, err := resultOptions(cmd, false)
	if err != nil {
		return err
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	cfg, err := config.ClientConfig()
	if err != nil {
		return err
	}
	if timeout > 0 {
		cfg.Timeout = timeout
	}
	c := client.New(cfg)

	output, _ := cmd.Flags().GetString("output")
	if err := validateParquetOutput(opts.Format, output, len(queries)); err != nil {
		return err
	}
	params, _ := cmd.Flags().GetStringArray("param")
	requests := make([]client.QueryRequest, len(queries))
	for i, query := range queries {
		query, err = config.ApplyParams(query, params)
		if err != nil {
			return err
		}
		requests[i], err = queryRequest(cmd, query)
		if err != nil {
			return err
		}
	}
	err = writeResults(output, opts.Format, func(w io.Writer) error {
		var failures []error
		for i, req := range requests {
			if err := executeRequest(cmd.Context(), w, c, req, opts); err != nil {
				if len(requests) == 1 {
					return err
				}
				failures = append(failures, fmt.Errorf("query %d: %w", i+1, err))
				continue
			}
		}
		return errors.Join(failures...)
	})
	if err == nil && output != "" && !opts.Quiet {
		fmt.Fprintf(os.Stderr, "Results written to %s\n", output)
	}
	return err
}

func executeRequest(ctx context.Context, w io.Writer, c *client.Client, request client.QueryRequest, opts formatter.Options) error {
	resp, err := c.ExecuteQuery(ctx, request)
	if err != nil {
		return err
	}
	if err := formatter.Format(w, resp, opts); err != nil {
		return err
	}
	if !opts.Quiet && formatter.IsTTY() {
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

// Parquet has one schema and footer per file. Concatenating query outputs would
// produce a corrupt file; validate before opening or truncating the destination.
func validateParquetOutput(format, output string, queryCount int) error {
	if !strings.EqualFold(format, "parquet") {
		return nil
	}
	if queryCount != 1 {
		return fmt.Errorf("Parquet output requires exactly one query; run each query with a separate output file")
	}
	if output == "" && formatter.IsTTY() {
		return fmt.Errorf("Parquet is binary; use --output or redirect stdout to a file")
	}
	return nil
}

// writeResults publishes a Parquet file only after the footer and file close
// succeed. Failures leave an existing destination intact and remove temp files.
func writeResults(output, format string, write func(io.Writer) error) error {
	if output == "" || !strings.EqualFold(format, "parquet") {
		w, cleanup, err := resolveWriter(output)
		if err != nil {
			return err
		}
		defer cleanup()
		return write(w)
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".esql-parquet-*")
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing output file: %w", err)
	}
	if err := os.Rename(f.Name(), output); err != nil {
		return fmt.Errorf("publishing output file: %w", err)
	}
	return nil
}
