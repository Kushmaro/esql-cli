package repl

import (
	"testing"
)

// ---------- Test 9: needsContinuation ----------

func TestNeedsContinuation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"ends with pipe", "FROM logs |", true},
		{"ends with backslash", "FROM logs \\", true},
		{"ends with normal text", "FROM logs | LIMIT 10", false},
		{"empty string", "", false},
		{"pipe with trailing whitespace", "FROM logs |  ", true},
		{"backslash with trailing whitespace", "FROM logs \\  ", true},
		{"just a pipe", "|", true},
		{"complete query", "FROM logs | STATS count = COUNT(*) BY host", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsContinuation(tt.input)
			if got != tt.want {
				t.Errorf("needsContinuation(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- Test 10: extractIndexName ----------

func TestExtractIndexName(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"simple FROM", "FROM logs-*", "logs-*"},
		{"FROM with pipe", "FROM logs-* | WHERE x > 1", "logs-*"},
		{"lowercase from", "from logs-*", "logs-*"},
		{"mixed case", "From my_index", "my_index"},
		{"no FROM clause", "ROW x = 1", ""},
		{"trailing comma", "FROM logs-*,", "logs-*"},
		{"empty string", "", ""},
		{"FROM with metadata", "FROM logs-* METADATA _id", "logs-*"},
		{"FROM only", "FROM ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIndexName(tt.query)
			if got != tt.want {
				t.Errorf("extractIndexName(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}
