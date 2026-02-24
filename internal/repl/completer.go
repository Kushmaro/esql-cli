package repl

import (
	"strings"
	"sync"

	"github.com/Kushmaro/esql-cli/internal/client"
)

type completer struct {
	client     *client.Client
	indexNames []string
	fieldCache map[string][]string
	mu         sync.RWMutex
}

func newCompleter(c *client.Client) *completer {
	return &completer{
		client:     c,
		fieldCache: make(map[string][]string),
	}
}

func (c *completer) loadIndexNames() {
	indices, err := c.client.ListIndices("")
	if err != nil {
		return
	}
	names := make([]string, 0, len(indices))
	for _, idx := range indices {
		if !strings.HasPrefix(idx.Index, ".") {
			names = append(names, idx.Index)
		}
	}

	streams, err := c.client.ListDataStreams("")
	if err == nil {
		for _, ds := range streams {
			names = append(names, ds.Name)
		}
	}

	resolved, err := c.client.ResolveIndex("")
	if err == nil {
		for _, alias := range resolved.Aliases {
			if !strings.HasPrefix(alias.Name, ".") {
				names = append(names, alias.Name)
			}
		}
	}

	seen := make(map[string]bool, len(names))
	deduped := make([]string, 0, len(names))
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			deduped = append(deduped, n)
		}
	}

	c.mu.Lock()
	c.indexNames = deduped
	c.mu.Unlock()
}

func (c *completer) getFieldNames(indexName string) []string {
	indexName = strings.TrimRight(indexName, " |,")
	if indexName == "" {
		return nil
	}

	c.mu.RLock()
	fields, ok := c.fieldCache[indexName]
	c.mu.RUnlock()
	if ok {
		return fields
	}

	go c.fetchFields(indexName)
	return nil
}

func (c *completer) fetchFields(indexName string) {
	c.mu.Lock()
	if _, ok := c.fieldCache[indexName]; ok {
		c.mu.Unlock()
		return
	}
	c.fieldCache[indexName] = nil // mark as in-flight
	c.mu.Unlock()

	fieldInfos, err := c.client.GetFields(indexName)
	if err != nil {
		c.mu.Lock()
		delete(c.fieldCache, indexName)
		c.mu.Unlock()
		return
	}

	names := make([]string, len(fieldInfos))
	for i, f := range fieldInfos {
		names[i] = f.Name
	}

	c.mu.Lock()
	c.fieldCache[indexName] = names
	c.mu.Unlock()
}

// prefetchFields fetches field names for an index synchronously (used after query execution).
func (c *completer) prefetchFields(indexName string) {
	c.mu.RLock()
	_, ok := c.fieldCache[indexName]
	c.mu.RUnlock()
	if ok {
		return
	}
	c.fetchFields(indexName)
}

// wordComplete implements liner.WordCompleter.
func (c *completer) wordComplete(line string, pos int) (string, []string, string) {
	left := line[:pos]
	right := line[pos:]

	wordStart := lastWordBoundary(left)
	prefix := left[wordStart:]
	head := left[:wordStart]

	candidates := c.getCandidates(left, head)

	initHighlightSets()

	var matches []string
	upperPrefix := strings.ToUpper(prefix)
	for _, cand := range candidates {
		if strings.HasPrefix(strings.ToUpper(cand), upperPrefix) {
			if functionSet[strings.ToUpper(cand)] {
				matches = append(matches, cand+"(")
			} else {
				matches = append(matches, cand)
			}
		}
	}

	return head, matches, right
}

func (c *completer) getCandidates(left, head string) []string {
	trimmed := strings.TrimSpace(left)

	if strings.HasPrefix(trimmed, "\\") {
		return metaCommandNames()
	}

	// Find the last pipe segment
	lastPipe := strings.LastIndex(trimmed, "|")
	segment := trimmed
	if lastPipe >= 0 {
		segment = strings.TrimSpace(trimmed[lastPipe+1:])
	}

	words := strings.Fields(segment)

	if len(words) == 0 {
		if lastPipe >= 0 {
			return processingCommands
		}
		return sourceCommands
	}

	cmd := strings.ToUpper(words[0])

	switch cmd {
	case "FROM":
		if len(words) == 1 || (len(words) == 2 && !strings.HasSuffix(left, " ")) {
			c.mu.RLock()
			names := c.indexNames
			c.mu.RUnlock()
			return names
		}
		if len(words) == 2 && strings.HasSuffix(left, " ") {
			return []string{"METADATA", "|"}
		}
		return []string{"|"}

	case "WHERE", "EVAL":
		fields := c.fieldsFromQuery(left)
		result := make([]string, 0, len(fields)+len(allFunctions)+len(clauses))
		result = append(result, fields...)
		result = append(result, allFunctions...)
		result = append(result, "AND", "OR", "NOT", "IN", "LIKE", "RLIKE", "IS NULL", "IS NOT NULL", "|")
		return result

	case "STATS", "INLINE":
		upperSeg := strings.ToUpper(segment)
		if strings.Contains(upperSeg, " BY ") {
			fields := c.fieldsFromQuery(left)
			result := make([]string, 0, len(fields)+len(groupingFunctions)+1)
			result = append(result, fields...)
			result = append(result, groupingFunctions...)
			result = append(result, "|")
			return result
		}
		fields := c.fieldsFromQuery(left)
		result := make([]string, 0, len(fields)+len(aggregationFunctions)+len(timeSeriesAggFunctions)+2)
		result = append(result, fields...)
		result = append(result, aggregationFunctions...)
		result = append(result, timeSeriesAggFunctions...)
		result = append(result, "BY", "|")
		return result

	case "SORT":
		fields := c.fieldsFromQuery(left)
		result := make([]string, 0, len(fields)+5)
		result = append(result, fields...)
		result = append(result, "ASC", "DESC", "NULLS FIRST", "NULLS LAST", "|")
		return result

	case "KEEP", "DROP":
		fields := c.fieldsFromQuery(left)
		return append(fields, "|")

	case "RENAME":
		fields := c.fieldsFromQuery(left)
		return append(fields, "AS", "|")

	case "LIMIT":
		return []string{"|"}

	case "ENRICH":
		return append(c.fieldsFromQuery(left), "ON", "WITH", "|")

	case "DISSECT", "GROK":
		return append(c.fieldsFromQuery(left), "|")

	case "LOOKUP":
		c.mu.RLock()
		names := c.indexNames
		c.mu.RUnlock()
		return append(names, "ON", "|")

	default:
		// Generic fallback: offer everything useful
		fields := c.fieldsFromQuery(left)
		result := make([]string, 0, len(processingCommands)+len(fields)+len(allFunctions))
		result = append(result, processingCommands...)
		result = append(result, fields...)
		result = append(result, allFunctions...)
		return result
	}
}

// fieldsFromQuery extracts the index name from a FROM clause and returns cached field names.
func (c *completer) fieldsFromQuery(fullQuery string) []string {
	idx := extractIndexName(fullQuery)
	if idx == "" {
		return nil
	}
	return c.getFieldNames(idx)
}

func extractIndexName(query string) string {
	upper := strings.ToUpper(query)
	fromIdx := strings.Index(upper, "FROM ")
	if fromIdx < 0 {
		return ""
	}

	rest := strings.TrimSpace(query[fromIdx+5:])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}

	name := fields[0]
	name = strings.TrimRight(name, "|,")
	return strings.TrimSpace(name)
}

func lastWordBoundary(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ' ', '\t', '|', ',', '(', ')':
			return i + 1
		}
	}
	return 0
}

func metaCommandNames() []string {
	return []string{
		"\\q", "\\f", "\\timing", "\\clear", "\\d", "\\h", "\\refresh",
		"\\sig", "\\sample", "\\s",
	}
}
