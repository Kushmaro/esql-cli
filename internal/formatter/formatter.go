package formatter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/term"
)

type Options struct {
	Format   string
	NoHeader bool
}

// IsTTY returns true if stdout is a terminal (not piped).
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// DefaultFormat returns "table" for TTY, "tsv" when piped.
func DefaultFormat() string {
	if IsTTY() {
		return "table"
	}
	return "tsv"
}

// Format writes the query response to w using the given options.
func Format(w io.Writer, resp *client.QueryResponse, opts Options) error {
	switch strings.ToLower(opts.Format) {
	case "table":
		return formatTable(w, resp, opts)
	case "json":
		return formatJSON(w, resp)
	case "csv":
		return formatCSV(w, resp, opts)
	case "tsv":
		return formatTSV(w, resp, opts)
	default:
		return fmt.Errorf("unknown format %q (supported: table, json, csv, tsv)", opts.Format)
	}
}

func formatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatTable(w io.Writer, resp *client.QueryResponse, opts Options) error {
	if len(resp.Columns) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}

	table := tablewriter.NewTable(w,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)

	if !opts.NoHeader {
		headers := make([]string, len(resp.Columns))
		for i, col := range resp.Columns {
			headers[i] = fmt.Sprintf("%s:%s", col.Name, col.Type)
		}
		table.Header(headers)
	}

	for _, row := range resp.Values {
		strRow := make([]any, len(row))
		for i, val := range row {
			strRow[i] = formatValue(val)
		}
		table.Append(strRow)
	}

	table.Render()
	return nil
}

func formatJSON(w io.Writer, resp *client.QueryResponse) error {
	if len(resp.Values) == 0 {
		fmt.Fprintln(w, "[]")
		return nil
	}

	rows := make([]map[string]interface{}, len(resp.Values))
	for i, vals := range resp.Values {
		row := make(map[string]interface{}, len(resp.Columns))
		for j, col := range resp.Columns {
			row[col.Name] = vals[j]
		}
		rows[i] = row
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func formatCSV(w io.Writer, resp *client.QueryResponse, opts Options) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if !opts.NoHeader {
		headers := make([]string, len(resp.Columns))
		for i, col := range resp.Columns {
			headers[i] = col.Name
		}
		if err := cw.Write(headers); err != nil {
			return err
		}
	}

	for _, row := range resp.Values {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = formatValue(v)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func formatTSV(w io.Writer, resp *client.QueryResponse, opts Options) error {
	if !opts.NoHeader {
		headers := make([]string, len(resp.Columns))
		for i, col := range resp.Columns {
			headers[i] = col.Name
		}
		fmt.Fprintln(w, strings.Join(headers, "\t"))
	}

	for _, row := range resp.Values {
		vals := make([]string, len(row))
		for i, v := range row {
			vals[i] = formatValue(v)
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return nil
}
