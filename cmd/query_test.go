package cmd

import (
	"testing"
)

func TestSplitQueries(t *testing.T) {
	t.Run("single query no semicolons", func(t *testing.T) {
		got := splitQueries("FROM logs | LIMIT 10")
		if len(got) != 1 {
			t.Fatalf("expected 1 query, got %d", len(got))
		}
		if got[0] != "FROM logs | LIMIT 10" {
			t.Errorf("got %q, want %q", got[0], "FROM logs | LIMIT 10")
		}
	})

	t.Run("two queries separated by semicolon", func(t *testing.T) {
		got := splitQueries("FROM logs | LIMIT 5; FROM metrics | LIMIT 3")
		if len(got) != 2 {
			t.Fatalf("expected 2 queries, got %d", len(got))
		}
		if got[0] != "FROM logs | LIMIT 5" {
			t.Errorf("query 1 = %q, want %q", got[0], "FROM logs | LIMIT 5")
		}
		if got[1] != "FROM metrics | LIMIT 3" {
			t.Errorf("query 2 = %q, want %q", got[1], "FROM metrics | LIMIT 3")
		}
	})

	t.Run("trailing semicolon", func(t *testing.T) {
		got := splitQueries("FROM logs | LIMIT 5;")
		if len(got) != 1 {
			t.Fatalf("expected 1 query (trailing ; ignored), got %d: %v", len(got), got)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := splitQueries("")
		if len(got) != 0 {
			t.Fatalf("expected 0 queries, got %d: %v", len(got), got)
		}
	})

	t.Run("whitespace-only segments skipped", func(t *testing.T) {
		got := splitQueries("FROM logs;   ;  ; FROM metrics")
		if len(got) != 2 {
			t.Fatalf("expected 2 queries (empty segments skipped), got %d: %v", len(got), got)
		}
	})

	t.Run("queries are trimmed", func(t *testing.T) {
		got := splitQueries("  FROM logs  ;  FROM metrics  ")
		if len(got) != 2 {
			t.Fatalf("expected 2 queries, got %d", len(got))
		}
		if got[0] != "FROM logs" {
			t.Errorf("query 1 = %q, want %q (should be trimmed)", got[0], "FROM logs")
		}
		if got[1] != "FROM metrics" {
			t.Errorf("query 2 = %q, want %q (should be trimmed)", got[1], "FROM metrics")
		}
	})
}
