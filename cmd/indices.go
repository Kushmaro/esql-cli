package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var indicesCmd = &cobra.Command{
	Use:   "indices [pattern]",
	Short: "List available indices",
	Long: `List indices on the connected Elasticsearch cluster.

Optionally provide a pattern to filter results (supports wildcards).

Examples:
  esql indices
  esql indices 'logs-*'
  esql indices '.kibana*' --format json`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runIndices,
}

func init() {
	indicesCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv")
	indicesCmd.Flags().Bool("no-header", false, "suppress column headers")
	rootCmd.AddCommand(indicesCmd)
}

func runIndices(cmd *cobra.Command, args []string) error {
	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}

	cfg := config.ClientConfig()
	c := client.New(cfg)

	indices, err := c.ListIndices(pattern)
	if err != nil {
		return err
	}

	if len(indices) == 0 {
		fmt.Fprintln(os.Stderr, "No indices found.")
		return nil
	}

	format, _ := cmd.Flags().GetString("format")
	if format == "" {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			format = "table"
		} else {
			format = "tsv"
		}
	}
	noHeader, _ := cmd.Flags().GetBool("no-header")

	switch strings.ToLower(format) {
	case "json":
		return indicesJSON(indices)
	case "csv":
		return indicesDelimited(indices, ",", noHeader)
	case "tsv":
		return indicesDelimited(indices, "\t", noHeader)
	default:
		return indicesTable(indices, noHeader)
	}
}

func indicesTable(indices []client.IndexInfo, noHeader bool) error {
	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)

	if !noHeader {
		tbl.Header([]string{"health", "status", "index", "docs.count", "store.size"})
	}

	for _, idx := range indices {
		tbl.Append([]any{idx.Health, idx.Status, idx.Index, idx.DocsCount, idx.StoreSize})
	}

	tbl.Render()
	return nil
}

func indicesJSON(indices []client.IndexInfo) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(indices)
}

func indicesDelimited(indices []client.IndexInfo, delim string, noHeader bool) error {
	if !noHeader {
		fmt.Println(strings.Join([]string{"health", "status", "index", "docs.count", "store.size"}, delim))
	}
	for _, idx := range indices {
		fmt.Println(strings.Join([]string{idx.Health, idx.Status, idx.Index, idx.DocsCount, idx.StoreSize}, delim))
	}
	return nil
}
