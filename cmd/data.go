package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type dataEntry struct {
	Type      string `json:"type"`
	Health    string `json:"health"`
	Name      string `json:"name"`
	DocsCount string `json:"docs.count"`
	StoreSize string `json:"store.size"`
}

var dataCmd = &cobra.Command{
	Use:   "data [name] [fields]",
	Short: "List indices and data streams, or inspect fields",
	Long: `List indices and data streams on the connected Elasticsearch cluster.
When called with a name and "fields", shows the field mapping for that
index or data stream.

Examples:
  esql data
  esql data 'logs-*'
  esql data logs-generic-default fields
  esql data --format json`,
	Args:         cobra.RangeArgs(0, 2),
	SilenceUsage: true,
	RunE:         runData,
}

func init() {
	dataCmd.Flags().StringP("format", "f", "", "output format: table, json, csv, tsv")
	dataCmd.Flags().Bool("no-header", false, "suppress column headers")
	rootCmd.AddCommand(dataCmd)
}

func runData(cmd *cobra.Command, args []string) error {
	if len(args) == 2 && strings.EqualFold(args[1], "fields") {
		return runDataFields(cmd, args[0])
	}

	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}

	cfg, err := config.ClientConfig()
	if err != nil {
		return err
	}
	c := client.New(cfg)

	entries, err := listData(c, pattern)
	if err != nil {
		if client.IsPrivilegeError(err) {
			return fmt.Errorf("%w\n\n%s", err, client.PrivilegeHint)
		}
		return err
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No indices or data streams found.")
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
		return dataJSON(entries)
	case "csv":
		return dataDelimited(entries, ",", noHeader)
	case "tsv":
		return dataDelimited(entries, "\t", noHeader)
	default:
		return dataTable(entries, noHeader)
	}
}

func listData(c *client.Client, pattern string) ([]dataEntry, error) {
	indices, idxErr := c.ListIndices(pattern)
	streams, dsErr := c.ListDataStreams(pattern)

	if idxErr != nil && dsErr != nil {
		if client.IsPrivilegeError(idxErr) {
			return listDataFallback(c, pattern)
		}
		return nil, idxErr
	}

	// Build reverse map: backing index name -> data stream name
	backingToDS := make(map[string]string)
	var dsNames []string
	if dsErr == nil {
		for _, ds := range streams {
			dsNames = append(dsNames, ds.Name)
			for _, name := range ds.BackingIndexNames {
				backingToDS[name] = ds.Name
			}
		}
	}

	// Resolve data stream names via _cat/indices to get backing index stats.
	// _cat/indices/<ds_name> automatically expands to backing indices.
	type dsStats struct{ docs, bytes int64 }
	statsMap := make(map[string]*dsStats)
	if len(dsNames) > 0 {
		backingIndices, err := c.ListIndices(strings.Join(dsNames, ","))
		if err == nil {
			for _, idx := range backingIndices {
				dsName := backingToDS[idx.Index]
				if dsName == "" {
					continue
				}
				if statsMap[dsName] == nil {
					statsMap[dsName] = &dsStats{}
				}
				if n, err := strconv.ParseInt(idx.DocsCount, 10, 64); err == nil {
					statsMap[dsName].docs += n
				}
				if n, err := strconv.ParseInt(idx.StoreSize, 10, 64); err == nil {
					statsMap[dsName].bytes += n
				}
			}
		}
	}

	var entries []dataEntry

	// Add regular indices, skip backing indices and system indices
	if idxErr == nil {
		for _, idx := range indices {
			if backingToDS[idx.Index] != "" || strings.HasPrefix(idx.Index, ".") {
				continue
			}
			entries = append(entries, dataEntry{
				Type:      "index",
				Health:    idx.Health,
				Name:      idx.Index,
				DocsCount: idx.DocsCount,
				StoreSize: displaySize(idx.StoreSize, idx.DocsCount),
			})
		}
	} else if client.IsPrivilegeError(idxErr) {
		// _cat/indices failed with 403 but _data_stream succeeded.
		// Fall back to _resolve/index to discover regular indices.
		resolved, err := c.ResolveIndex(pattern)
		if err == nil {
			dsSet := make(map[string]bool, len(dsNames))
			for _, n := range dsNames {
				dsSet[n] = true
			}
			for _, idx := range resolved.Indices {
				if strings.HasPrefix(idx.Name, ".") || dsSet[idx.Name] {
					continue
				}
				entries = append(entries, dataEntry{
					Type:      "index",
					Name:      idx.Name,
					DocsCount: "n/a",
					StoreSize: "n/a",
				})
			}
		}
	}

	// Add data streams with aggregated stats
	if dsErr == nil {
		for _, ds := range streams {
			docs := "n/a"
			size := "n/a"
			if s := statsMap[ds.Name]; s != nil {
				docs = strconv.FormatInt(s.docs, 10)
				size = displaySize(strconv.FormatInt(s.bytes, 10), docs)
			}
			entries = append(entries, dataEntry{
				Type:      "data_stream",
				Health:    ds.Status,
				Name:      ds.Name,
				DocsCount: docs,
				StoreSize: size,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

// listDataFallback uses _resolve/index when _cat/indices is forbidden.
func listDataFallback(c *client.Client, pattern string) ([]dataEntry, error) {
	resolved, err := c.ResolveIndex(pattern)
	if err != nil {
		return nil, fmt.Errorf("insufficient privileges to list indices.\n\n%s", client.PrivilegeHint)
	}

	var entries []dataEntry
	dsSet := make(map[string]bool)
	for _, ds := range resolved.DataStreams {
		dsSet[ds.Name] = true
		entries = append(entries, dataEntry{
			Type:      "data_stream",
			Name:      ds.Name,
			DocsCount: "n/a",
			StoreSize: "n/a",
		})
	}

	for _, idx := range resolved.Indices {
		if strings.HasPrefix(idx.Name, ".") || dsSet[idx.Name] {
			continue
		}
		entries = append(entries, dataEntry{
			Type:      "index",
			Name:      idx.Name,
			DocsCount: "n/a",
			StoreSize: "n/a",
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

// displaySize formats byte count for display, showing "n/a" when the cluster
// reports 0 bytes but docs exist (common on serverless clusters).
func displaySize(sizeBytes, docsCount string) string {
	if sizeBytes == "0" || sizeBytes == "" {
		if n, err := strconv.ParseInt(docsCount, 10, 64); err == nil && n > 0 {
			return "n/a"
		}
	}
	return formatBytes(sizeBytes)
}

func formatBytes(s string) string {
	b, err := strconv.ParseInt(s, 10, 64)
	if err != nil || b < 0 {
		return s
	}
	switch {
	case b < 1024:
		return fmt.Sprintf("%db", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1fkb", float64(b)/1024)
	case b < 1024*1024*1024:
		return fmt.Sprintf("%.1fmb", float64(b)/(1024*1024))
	case b < 1024*1024*1024*1024:
		return fmt.Sprintf("%.1fgb", float64(b)/(1024*1024*1024))
	default:
		return fmt.Sprintf("%.1ftb", float64(b)/(1024*1024*1024*1024))
	}
}

var dataHeaders = []string{"type", "health", "name", "docs.count", "store.size"}

func dataTable(entries []dataEntry, noHeader bool) error {
	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)

	if !noHeader {
		tbl.Header(dataHeaders)
	}

	for _, e := range entries {
		tbl.Append([]any{e.Type, e.Health, e.Name, e.DocsCount, e.StoreSize})
	}

	tbl.Render()
	return nil
}

func dataJSON(entries []dataEntry) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func dataDelimited(entries []dataEntry, delim string, noHeader bool) error {
	if !noHeader {
		fmt.Println(strings.Join(dataHeaders, delim))
	}
	for _, e := range entries {
		fmt.Println(strings.Join([]string{e.Type, e.Health, e.Name, e.DocsCount, e.StoreSize}, delim))
	}
	return nil
}

func runDataFields(cmd *cobra.Command, index string) error {
	cfg, err := config.ClientConfig()
	if err != nil {
		return err
	}
	c := client.New(cfg)

	fields, err := c.GetFields(index)
	if err != nil {
		if client.IsPrivilegeError(err) {
			return fmt.Errorf("%w\n\n%s", err, client.PrivilegeHint)
		}
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
