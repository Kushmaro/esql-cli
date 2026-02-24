package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [esql query string]",
	Short: "Execute an ES|QL query",
	Long: `Execute an ES|QL query against the configured Elasticsearch cluster
and display the results.

Example:
  esql query 'FROM logs-* | WHERE status >= 400 | LIMIT 10'
  esql query 'FROM employees | STATS count = COUNT(*) BY department'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		format, _ := cmd.Flags().GetString("format")

		_ = query
		_ = format

		fmt.Println("query command not yet implemented")
		return nil
	},
}

func init() {
	queryCmd.Flags().StringP("format", "f", "table", "output format: table, json, csv")
	rootCmd.AddCommand(queryCmd)
}
