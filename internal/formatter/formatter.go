package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/olekukonko/tablewriter"
)

// Format writes the query response to w in the given format.
func Format(w io.Writer, resp *client.QueryResponse, format string) error {
	switch strings.ToLower(format) {
	case "table":
		return formatTable(w, resp)
	case "json":
		return formatJSON(w, resp)
	case "csv":
		return formatCSV(w, resp)
	default:
		return fmt.Errorf("unknown format %q (supported: table, json, csv)", format)
	}
}

func formatTable(w io.Writer, resp *client.QueryResponse) error {
	headers := make([]string, len(resp.Columns))
	for i, col := range resp.Columns {
		headers[i] = fmt.Sprintf("%s:%s", col.Name, col.Type)
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader(headers)
	table.SetAutoFormatHeaders(false)
	table.SetBorder(true)

	for _, row := range resp.Values {
		strRow := make([]string, len(row))
		for i, val := range row {
			strRow[i] = fmt.Sprintf("%v", val)
		}
		table.Append(strRow)
	}

	table.Render()
	return nil
}

func formatJSON(w io.Writer, resp *client.QueryResponse) error {
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

func formatCSV(w io.Writer, resp *client.QueryResponse) error {
	headers := make([]string, len(resp.Columns))
	for i, col := range resp.Columns {
		headers[i] = col.Name
	}
	fmt.Fprintln(w, strings.Join(headers, ","))

	for _, row := range resp.Values {
		vals := make([]string, len(row))
		for i, v := range row {
			vals[i] = fmt.Sprintf("%v", v)
		}
		fmt.Fprintln(w, strings.Join(vals, ","))
	}
	return nil
}
