package client

import (
	"strings"
	"testing"
)

func TestErrorHint(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if got := ErrorHint(nil); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("401 unauthorized", func(t *testing.T) {
		err := &ESError{Status: 401, Type: "security_exception", Reason: "missing credentials"}
		got := ErrorHint(err)
		if !strings.Contains(got, "401") || !strings.Contains(got, "Authentication") {
			t.Errorf("hint = %q, expected auth-related hint for 401", got)
		}
	})

	t.Run("403 forbidden", func(t *testing.T) {
		err := &ESError{Status: 403, Type: "security_exception", Reason: "forbidden"}
		got := ErrorHint(err)
		if !strings.Contains(got, "403") || !strings.Contains(got, "privileges") {
			t.Errorf("hint = %q, expected privilege hint for 403", got)
		}
	})

	t.Run("404 no such index", func(t *testing.T) {
		err := &ESError{Status: 404, Type: "index_not_found_exception", Reason: "no such index [missing]"}
		got := ErrorHint(err)
		if !strings.Contains(got, "Index not found") {
			t.Errorf("hint = %q, expected index-not-found hint", got)
		}
	})

	t.Run("404 generic", func(t *testing.T) {
		err := &ESError{Status: 404, Type: "resource_not_found", Reason: "something missing"}
		got := ErrorHint(err)
		if !strings.Contains(got, "404") {
			t.Errorf("hint = %q, expected generic 404 hint", got)
		}
	})

	t.Run("400 parsing_exception", func(t *testing.T) {
		err := &ESError{Status: 400, Type: "parsing_exception", Reason: "Unknown command [FORM]"}
		got := ErrorHint(err)
		if !strings.Contains(got, "syntax") || !strings.Contains(got, "ES|QL") {
			t.Errorf("hint = %q, expected syntax error hint for parsing_exception", got)
		}
	})

	t.Run("connection refused via ConnectionError", func(t *testing.T) {
		inner := &ConnectionError{Err: &stubError{msg: "dial tcp 127.0.0.1:9200: connection refused"}}
		got := ErrorHint(inner)
		if !strings.Contains(got, "Connection refused") {
			t.Errorf("hint = %q, expected connection refused hint", got)
		}
	})

	t.Run("TLS error via string match", func(t *testing.T) {
		inner := &ConnectionError{Err: &stubError{msg: "tls: failed to verify certificate"}}
		got := ErrorHint(inner)
		if !strings.Contains(got, "TLS") && !strings.Contains(got, "certificate") {
			t.Errorf("hint = %q, expected TLS/certificate hint", got)
		}
	})

	t.Run("timeout error via string match", func(t *testing.T) {
		inner := &ConnectionError{Err: &stubError{msg: "context deadline exceeded"}}
		got := ErrorHint(inner)
		if !strings.Contains(got, "timed out") {
			t.Errorf("hint = %q, expected timeout hint", got)
		}
	})

	t.Run("503 service unavailable", func(t *testing.T) {
		err := &ESError{Status: 503, Type: "", Reason: ""}
		got := ErrorHint(err)
		if !strings.Contains(got, "503") {
			t.Errorf("hint = %q, expected 503 hint", got)
		}
	})

	t.Run("429 too many requests", func(t *testing.T) {
		err := &ESError{Status: 429, Type: "", Reason: ""}
		got := ErrorHint(err)
		if !strings.Contains(got, "429") {
			t.Errorf("hint = %q, expected 429 hint", got)
		}
	})
}

func TestParsingSuggestion(t *testing.T) {
	t.Run("typo suggestion", func(t *testing.T) {
		reason := "mismatched input 'WHER' expecting {'WHERE', 'EVAL', 'STATS'}"
		got := parsingSuggestion(reason)
		if !strings.Contains(got, "WHERE") {
			t.Errorf("suggestion = %q, expected 'Did you mean WHERE?'", got)
		}
	})

	t.Run("unknown function", func(t *testing.T) {
		reason := "Unknown function [AVGG]"
		got := parsingSuggestion(reason)
		if !strings.Contains(got, "AVGG") {
			t.Errorf("suggestion = %q, expected mention of AVGG", got)
		}
	})

	t.Run("unknown column", func(t *testing.T) {
		reason := "Unknown column [hostt]"
		got := parsingSuggestion(reason)
		if !strings.Contains(got, "hostt") {
			t.Errorf("suggestion = %q, expected mention of hostt", got)
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		got := parsingSuggestion("some unrelated error reason")
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

// stubError is a minimal error implementation for testing string-based hint matching.
type stubError struct {
	msg string
}

func (e *stubError) Error() string { return e.msg }
