package config

import (
	"strings"
	"testing"
)

func TestApplyParams(t *testing.T) {
	t.Run("single placeholder", func(t *testing.T) {
		got, err := ApplyParams("FROM {{idx}}", []string{"idx=logs"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "FROM logs" {
			t.Errorf("got %q, want %q", got, "FROM logs")
		}
	})

	t.Run("multiple placeholders", func(t *testing.T) {
		got, err := ApplyParams(
			"FROM {{idx}} | WHERE status >= {{min}}",
			[]string{"idx=logs-*", "min=500"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "FROM logs-* | WHERE status >= 500"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no params returns query unchanged", func(t *testing.T) {
		query := "FROM logs | LIMIT 10"
		got, err := ApplyParams(query, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != query {
			t.Errorf("got %q, want %q", got, query)
		}
	})

	t.Run("empty params slice returns query unchanged", func(t *testing.T) {
		query := "FROM logs | LIMIT 10"
		got, err := ApplyParams(query, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != query {
			t.Errorf("got %q, want %q", got, query)
		}
	})

	t.Run("invalid param format missing equals", func(t *testing.T) {
		_, err := ApplyParams("FROM {{idx}}", []string{"idx"})
		if err == nil {
			t.Fatal("expected error for invalid param format")
		}
		if !strings.Contains(err.Error(), "invalid --param") {
			t.Errorf("error %q should mention 'invalid --param'", err)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := ApplyParams("FROM {{idx}}", []string{"=value"})
		if err == nil {
			t.Fatal("expected error for empty key")
		}
		if !strings.Contains(err.Error(), "invalid --param") {
			t.Errorf("error %q should mention 'invalid --param'", err)
		}
	})

	t.Run("unresolved placeholder", func(t *testing.T) {
		_, err := ApplyParams("FROM {{idx}} | WHERE status >= {{min}}", []string{"idx=logs"})
		if err == nil {
			t.Fatal("expected error for unresolved placeholder")
		}
		if !strings.Contains(err.Error(), "unresolved") {
			t.Errorf("error %q should mention 'unresolved'", err)
		}
		if !strings.Contains(err.Error(), "min") {
			t.Errorf("error %q should mention the missing key 'min'", err)
		}
	})

	t.Run("unused param key", func(t *testing.T) {
		_, err := ApplyParams("FROM {{idx}}", []string{"idx=logs", "extra=value"})
		if err == nil {
			t.Fatal("expected error for unused param key")
		}
		if !strings.Contains(err.Error(), "extra") {
			t.Errorf("error %q should mention the unused key 'extra'", err)
		}
	})
}
