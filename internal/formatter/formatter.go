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
	Raw       bool
	Format    string
	NoHeader  bool
	Delimiter string
	Columns   []string
	Quiet     bool
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
	if opts.Raw {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if len(resp.Raw) > 0 {
			return enc.Encode(resp.Raw)
		}
		return enc.Encode(resp)
	}
	if resp.IsRunning {
		return fmt.Errorf("query is still running; retrieve its results again before formatting rows")
	}
	// Validate before filtering, which indexes each selected value.
	if strings.EqualFold(opts.Format, "parquet") {
		for i, row := range resp.Values {
			if len(row) != len(resp.Columns) {
				return fmt.Errorf("Parquet row %d has %d values for %d columns", i+1, len(row), len(resp.Columns))
			}
		}
	}
	filtered := resp
	if len(opts.Columns) > 0 {
		filtered = filterColumns(resp, opts.Columns)
	}

	switch strings.ToLower(opts.Format) {
	case "table":
		return formatTable(w, filtered, opts)
	case "json":
		return formatJSON(w, filtered)
	case "ndjson":
		return formatNDJSON(w, filtered)
	case "csv":
		return formatCSV(w, filtered, opts)
	case "tsv":
		return formatDelimited(w, filtered, opts, "\t")
	case "parquet":
		return formatParquet(w, filtered)
	default:
		return fmt.Errorf("unknown format %q (supported: table, json, ndjson, csv, tsv, parquet)", opts.Format)
	}
}

func filterColumns(resp *client.QueryResponse, columns []string) *client.QueryResponse {
	keep := make(map[string]bool, len(columns))
	for _, c := range columns {
		keep[strings.TrimSpace(c)] = true
	}

	var indices []int
	var cols []client.Column
	for i, col := range resp.Columns {
		if keep[col.Name] {
			indices = append(indices, i)
			cols = append(cols, col)
		}
	}

	if len(indices) == 0 {
		return resp
	}

	values := make([][]interface{}, len(resp.Values))
	for i, row := range resp.Values {
		newRow := make([]interface{}, len(indices))
		for j, idx := range indices {
			newRow[j] = row[idx]
		}
		values[i] = newRow
	}

	return &client.QueryResponse{Columns: cols, Values: values, Took: resp.Took, IsPartial: resp.IsPartial}
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

func formatNDJSON(w io.Writer, resp *client.QueryResponse) error {
	enc := json.NewEncoder(w)
	for _, vals := range resp.Values {
		row := make(map[string]interface{}, len(resp.Columns))
		for j, col := range resp.Columns {
			row[col.Name] = vals[j]
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func formatDelimited(w io.Writer, resp *client.QueryResponse, opts Options, defaultDelim string) error {
	delim := defaultDelim
	if opts.Delimiter != "" {
		delim = opts.Delimiter
	}

	if !opts.NoHeader {
		headers := make([]string, len(resp.Columns))
		for i, col := range resp.Columns {
			headers[i] = col.Name
		}
		fmt.Fprintln(w, strings.Join(headers, delim))
	}

	for _, row := range resp.Values {
		vals := make([]string, len(row))
		for i, v := range row {
			vals[i] = formatValue(v)
		}
		fmt.Fprintln(w, strings.Join(vals, delim))
	}
	return nil
}
