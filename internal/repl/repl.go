package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			r.showIndices()
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

func (r *REPL) showIndices() {
	r.completer.mu.RLock()
	names := r.completer.indexNames
	r.completer.mu.RUnlock()

	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "No indices loaded. Try \\refresh")
		return
	}

	indices, err := r.client.ListIndices("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	tbl := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
			},
		}),
	)
	tbl.Header([]string{"health", "status", "index", "docs.count", "store.size"})

	for _, idx := range indices {
		if !strings.HasPrefix(idx.Index, ".") {
			tbl.Append([]any{idx.Health, idx.Status, idx.Index, idx.DocsCount, idx.StoreSize})
		}
	}
	tbl.Render()
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
  \d            List indices
  \d INDEX      Show fields for an index
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
