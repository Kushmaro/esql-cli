package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/exporter"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <index,alias,data-stream or pattern>",
	Short: "Export every document in an index to Parquet",
	Long: `Export all documents visible in a point-in-time snapshot, with no ES|QL
row limit. Supports index patterns, aliases, data streams, and comma-separated
indices. Requires read privileges and enabled _source.

Source fields become columns, with _index and _id metadata. Objects and fields
with conflicting types are JSON strings; arrays become Parquet lists. Schema
discovery uses a temporary disk spool, so disk space must accommodate the source
data plus the output file. The output is published only after success.

Examples:
  esql export 'logs-*' --output logs.parquet
  esql export 'index-a,index-b' -o data.parquet --page-size 500`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireConnection(cmd); err != nil {
			return err
		}
		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			return fmt.Errorf("--output is required for complete index exports")
		}
		pageSize, _ := cmd.Flags().GetInt("page-size")
		if pageSize < 1 || pageSize > 10000 {
			return fmt.Errorf("page size must be between 1 and 10000")
		}
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		cfg, err := config.ClientConfig()
		if err != nil {
			return err
		}
		cfg.Timeout = timeout
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		var count int64
		err = writeResults(output, "parquet", func(w io.Writer) error {
			var exportErr error
			count, exportErr = exporter.Export(ctx, client.New(cfg), args[0], pageSize, w)
			return exportErr
		})
		if err != nil {
			return err
		}
		quiet, _ := cmd.Flags().GetBool("quiet")
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Exported %d documents to %s\n", count, output)
		}
		return nil
	},
}

func init() {
	exportCmd.Flags().StringP("output", "o", "", "destination Parquet file (required)")
	exportCmd.Flags().Int("page-size", 1000, "documents per search request (1-10000)")
	exportCmd.Flags().Duration("timeout", 60*time.Second, "timeout per HTTP request")
	exportCmd.Flags().Bool("quiet", false, "suppress export summary")
	rootCmd.AddCommand(exportCmd)
}
