package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var fieldsCmd = &cobra.Command{
	Use:   "fields <index>",
	Short: "Show field names and types for an index",
	Long: `Show the field mapping for an index, data stream, or alias.

Fields are displayed in alphabetical order with their Elasticsearch type.
Nested fields are shown with dot notation (e.g. "host.name").

Examples:
  esql fields logs-2025.01
  esql fields my-index --format json
  esql fields 'logs-*' -f csv --no-header`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runFields,
}

func init() {
	fieldsCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv")
	fieldsCmd.Flags().Bool("no-header", false, "suppress column headers")
	rootCmd.AddCommand(fieldsCmd)
}

func runFields(cmd *cobra.Command, args []string) error {
	index := args[0]

	cfg := config.ClientConfig()
	c := client.New(cfg)

	fields, err := c.GetFields(index)
	if err != nil {
		return err
	}

	if len(fields) == 0 {
		fmt.Fprintln(os.Stderr, "No fields found.")
		return nil
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

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
		return fieldsJSON(fields)
	case "csv":
		return fieldsDelimited(fields, ",", noHeader)
	case "tsv":
		return fieldsDelimited(fields, "\t", noHeader)
	default:
		return fieldsTable(fields, noHeader)
	}
}

func fieldsTable(fields []client.FieldInfo, noHeader bool) error {
	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)

	if !noHeader {
		tbl.Header([]string{"field", "type"})
	}

	for _, f := range fields {
		tbl.Append([]any{f.Name, f.Type})
	}

	tbl.Render()
	return nil
}

func fieldsJSON(fields []client.FieldInfo) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(fields)
}

func fieldsDelimited(fields []client.FieldInfo, delim string, noHeader bool) error {
	if !noHeader {
		fmt.Println(strings.Join([]string{"field", "type"}, delim))
	}
	for _, f := range fields {
		fmt.Println(strings.Join([]string{f.Name, f.Type}, delim))
	}
	return nil
}
