package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/peterh/liner"
)

type REPL struct {
	client     *client.Client
	completer  *completer
	format     string
	showTiming bool
	buffer     strings.Builder
	continued  bool
}

func New(cfg client.Config) *REPL {
	c := client.New(cfg)
	return &REPL{
		client:     c,
		completer:  newCompleter(c),
		format:     "table",
		showTiming: true,
	}
}

func (r *REPL) Run() error {
	info, err := r.client.Ping()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Connected to %s/%s (cluster: %s)\n", info.Name, info.Version.Number, info.Cluster)
	fmt.Fprintln(os.Stderr, "Type \\h for help, \\q to quit.")
	fmt.Fprintln(os.Stderr)

	r.completer.loadIndexNames()

	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)
	line.SetWordCompleter(r.completer.wordComplete)
	line.SetTabCompletionStyle(liner.TabCircular)

	r.loadHistory(line)
	defer r.saveHistory(line)

	for {
		prompt := "esql> "
		if r.continued {
			prompt = "  ...> "
		}

		input, err := line.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			// Ctrl-C: cancel current input
			r.buffer.Reset()
			r.continued = false
			fmt.Fprintln(os.Stderr)
			continue
		}
		if err != nil {
			// Ctrl-D or EOF
			fmt.Fprintln(os.Stderr, "Bye!")
			return nil
		}

		if r.continued {
			r.buffer.WriteString("\n")
		}
		r.buffer.WriteString(input)

		text := r.buffer.String()

		if needsContinuation(text) {
			r.continued = true
			continue
		}

		r.continued = false
		fullInput := strings.TrimSpace(text)
		r.buffer.Reset()

		if fullInput == "" {
			continue
		}

		line.AppendHistory(fullInput)

		if strings.HasPrefix(fullInput, "\\") {
			if r.handleMeta(fullInput) {
				return nil
			}
			continue
		}

		if strings.EqualFold(fullInput, "exit") || strings.EqualFold(fullInput, "quit") {
			fmt.Fprintln(os.Stderr, "Bye!")
			return nil
		}

		r.executeQuery(fullInput)
	}
}

func (r *REPL) executeQuery(query string) {
	// Pre-fetch fields for the referenced index (for future completions)
	if idx := extractIndexName(query); idx != "" {
		go r.completer.prefetchFields(idx)
	}

	start := time.Now()
	resp, err := r.client.Query(query)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	opts := formatter.Options{
		Format: r.format,
	}
	if err := formatter.Format(os.Stdout, resp, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %s\n", err)
		return
	}

	if r.showTiming {
		fmt.Fprintf(os.Stderr, "\n%d rows in set (%dms, round-trip: %s)\n", len(resp.Values), resp.Took, elapsed.Round(time.Millisecond))
	} else {
		fmt.Fprintf(os.Stderr, "\n%d rows in set\n", len(resp.Values))
	}
}

// handleMeta processes backslash commands. Returns true if the REPL should exit.
func (r *REPL) handleMeta(input string) bool {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "\\q":
		fmt.Fprintln(os.Stderr, "Bye!")
		return true

	case "\\f":
		if len(parts) < 2 {
			fmt.Fprintf(os.Stderr, "Current format: %s\n", r.format)
			fmt.Fprintln(os.Stderr, "Usage: \\f table|json|csv|tsv")
			return false
		}
		f := strings.ToLower(parts[1])
		switch f {
		case "table", "json", "csv", "tsv":
			r.format = f
			fmt.Fprintf(os.Stderr, "Output format: %s\n", f)
		default:
			fmt.Fprintf(os.Stderr, "Unknown format %q (table, json, csv, tsv)\n", f)
		}

	case "\\timing":
		r.showTiming = !r.showTiming
		if r.showTiming {
			fmt.Fprintln(os.Stderr, "Timing is on.")
		} else {
			fmt.Fprintln(os.Stderr, "Timing is off.")
		}

	case "\\clear":
		fmt.Print("\033[H\033[2J")

	case "\\d":
		if len(parts) >= 2 {
			r.showFields(parts[1])
		} else {
			r.showData()
		}

	case "\\h", "\\?":
		r.showHelp()

	case "\\refresh":
		fmt.Fprintln(os.Stderr, "Refreshing index list...")
		r.completer.loadIndexNames()
		r.completer.mu.Lock()
		r.completer.fieldCache = make(map[string][]string)
		r.completer.mu.Unlock()
		fmt.Fprintf(os.Stderr, "Loaded %d indices/data streams\n", len(r.completer.indexNames))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s (type \\h for help)\n", cmd)
	}

	return false
}

func (r *REPL) showData() {
	type entry struct {
		kind, health, name, docs, size string
	}

	indices, idxErr := r.client.ListIndices("")
	streams, dsErr := r.client.ListDataStreams("")

	if idxErr != nil && dsErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", idxErr)
		return
	}

	// Reverse map: backing index name -> data stream name
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

	// Resolve data stream names via _cat/indices to get backing index stats
	type dsStats struct{ docs, bytes int64 }
	statsMap := make(map[string]*dsStats)
	if len(dsNames) > 0 {
		backingIndices, err := r.client.ListIndices(strings.Join(dsNames, ","))
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

	var entries []entry

	if idxErr == nil {
		for _, idx := range indices {
			if backingToDS[idx.Index] != "" || strings.HasPrefix(idx.Index, ".") {
				continue
			}
			entries = append(entries, entry{"index", idx.Health, idx.Index, idx.DocsCount, displaySize(idx.StoreSize, idx.DocsCount)})
		}
	}

	if dsErr == nil {
		for _, ds := range streams {
			docs := "0"
			size := "n/a"
			if s := statsMap[ds.Name]; s != nil {
				docs = strconv.FormatInt(s.docs, 10)
				size = displaySize(strconv.FormatInt(s.bytes, 10), docs)
			}
			entries = append(entries, entry{"data_stream", ds.Status, ds.Name, docs, size})
		}
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No indices or data streams found.")
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)
	tbl.Header([]string{"type", "health", "name", "docs.count", "store.size"})

	for _, e := range entries {
		tbl.Append([]any{e.kind, e.health, e.name, e.docs, e.size})
	}
	tbl.Render()
}

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

func (r *REPL) showFields(index string) {
	fields, err := r.client.GetFields(index)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	if len(fields) == 0 {
		fmt.Fprintf(os.Stderr, "No fields found for %q\n", index)
		return
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	// Also cache the fields for completion
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	r.completer.mu.Lock()
	r.completer.fieldCache[index] = names
	r.completer.mu.Unlock()

	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)
	tbl.Header([]string{"field", "type"})

	for _, f := range fields {
		tbl.Append([]any{f.Name, f.Type})
	}
	tbl.Render()
}

func (r *REPL) showHelp() {
	help := `
  ES|QL REPL Commands:

  \q            Quit
  \f FORMAT     Set output format (table, json, csv, tsv)
  \timing       Toggle query timing display
  \clear        Clear the screen
  \d            List indices and data streams
  \d INDEX      Show fields for an index or data stream
  \refresh      Refresh index list and clear field cache
  \h            Show this help

  Multiline queries:
    End a line with | or \ to continue on the next line.

  Keyboard shortcuts:
    Tab           Autocomplete
    Ctrl-C        Cancel current input
    Ctrl-D        Quit
    Up/Down       Navigate history
`
	fmt.Fprint(os.Stderr, help)
}

func needsContinuation(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasSuffix(trimmed, "\\") {
		return true
	}
	if strings.HasSuffix(trimmed, "|") {
		return true
	}
	return false
}

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".esql_history")
}

func (r *REPL) loadHistory(line *liner.State) {
	path := historyPath()
	if path == "" {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	line.ReadHistory(f)
}

func (r *REPL) saveHistory(line *liner.State) {
	path := historyPath()
	if path == "" {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	line.WriteHistory(f)
}
