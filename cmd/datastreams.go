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

var datastreamsCmd = &cobra.Command{
	Use:     "datastreams [pattern]",
	Aliases: []string{"ds"},
	Short:   "List data streams",
	Long: `List data streams on the connected Elasticsearch cluster.

Data streams are the primary way to query time-series data in ES|QL
(e.g. FROM logs-generic-default). This command shows the logical
data stream names you can use in your queries.

Optionally provide a pattern to filter results (supports wildcards).

Examples:
  esql datastreams
  esql ds
  esql datastreams 'logs-*'
  esql datastreams --format json`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runDataStreams,
}

func init() {
	datastreamsCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv")
	datastreamsCmd.Flags().Bool("no-header", false, "suppress column headers")
	rootCmd.AddCommand(datastreamsCmd)
}

func runDataStreams(cmd *cobra.Command, args []string) error {
	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}

	cfg := config.ClientConfig()
	c := client.New(cfg)

	streams, err := c.ListDataStreams(pattern)
	if err != nil {
		return err
	}

	if len(streams) == 0 {
		fmt.Fprintln(os.Stderr, "No data streams found.")
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
		return dsJSON(streams)
	case "csv":
		return dsDelimited(streams, ",", noHeader)
	case "tsv":
		return dsDelimited(streams, "\t", noHeader)
	default:
		return dsTable(streams, noHeader)
	}
}

var dsHeaders = []string{"health", "name", "index_mode", "backing_indices", "template"}

func dsTable(streams []client.DataStreamInfo, noHeader bool) error {
	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)

	if !noHeader {
		tbl.Header(dsHeaders)
	}

	for _, ds := range streams {
		tbl.Append([]any{ds.Status, ds.Name, ds.IndexMode, ds.BackingIndices, ds.Template})
	}

	tbl.Render()
	return nil
}

func dsJSON(streams []client.DataStreamInfo) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(streams)
}

func dsDelimited(streams []client.DataStreamInfo, delim string, noHeader bool) error {
	if !noHeader {
		fmt.Println(strings.Join(dsHeaders, delim))
	}
	for _, ds := range streams {
		fmt.Println(strings.Join([]string{
			ds.Status, ds.Name, ds.IndexMode,
			fmt.Sprint(ds.BackingIndices), ds.Template,
		}, delim))
	}
	return nil
}
