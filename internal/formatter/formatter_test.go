package formatter

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Kushmaro/esql-cli/internal/client"
)

// helper to build a simple response
func makeResp(cols []client.Column, rows [][]interface{}) *client.QueryResponse {
	return &client.QueryResponse{Columns: cols, Values: rows, Took: 42}
}

var sampleCols = []client.Column{
	{Name: "name", Type: "keyword"},
	{Name: "age", Type: "integer"},
}

var sampleRows = [][]interface{}{
	{"Alice", float64(30)},
	{"Bob", float64(25)},
}

// ---------- Test 2: JSON output ----------

func TestFormatJSON(t *testing.T) {
	t.Run("standard response", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var got []map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(got))
		}
		if got[0]["name"] != "Alice" {
			t.Errorf("got name=%v, want Alice", got[0]["name"])
		}
		if got[1]["age"] != float64(25) {
			t.Errorf("got age=%v, want 25", got[1]["age"])
		}
	})

	t.Run("empty result set", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, nil)
		if err := Format(&buf, resp, Options{Format: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(buf.String()) != "[]" {
			t.Errorf("got %q, want []", buf.String())
		}
	})

	t.Run("null values", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, [][]interface{}{{"Alice", nil}})
		if err := Format(&buf, resp, Options{Format: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var got []map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got[0]["age"] != nil {
			t.Errorf("expected null, got %v", got[0]["age"])
		}
	})

	t.Run("integer-like floats", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(
			[]client.Column{{Name: "count", Type: "long"}},
			[][]interface{}{{float64(42)}},
		)
		if err := Format(&buf, resp, Options{Format: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// json.Encoder will encode float64(42) as 42, but let's verify the round-trip
		var got []map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got[0]["count"] != float64(42) {
			t.Errorf("got %v, want 42", got[0]["count"])
		}
	})

	t.Run("multivalue fields", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(
			[]client.Column{{Name: "tags", Type: "keyword"}},
			[][]interface{}{{[]interface{}{"a", "b", "c"}}},
		)
		if err := Format(&buf, resp, Options{Format: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var got []map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		tags, ok := got[0]["tags"].([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T", got[0]["tags"])
		}
		if len(tags) != 3 || tags[0] != "a" {
			t.Errorf("got tags=%v, want [a b c]", tags)
		}
	})
}

// ---------- Test 3: CSV, TSV, NDJSON output ----------

func TestFormatCSV(t *testing.T) {
	t.Run("csv with headers", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "csv"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines (header + 2 rows), got %d: %v", len(lines), lines)
		}
		if lines[0] != "name,age" {
			t.Errorf("header = %q, want %q", lines[0], "name,age")
		}
		if lines[1] != "Alice,30" {
			t.Errorf("row 1 = %q, want %q", lines[1], "Alice,30")
		}
	})

	t.Run("csv no header", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "csv", NoHeader: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines (no header), got %d", len(lines))
		}
		if lines[0] != "Alice,30" {
			t.Errorf("row 1 = %q, want %q", lines[0], "Alice,30")
		}
	})
}

func TestFormatTSV(t *testing.T) {
	t.Run("tsv default tab delimiter", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "tsv"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
		if lines[0] != "name\tage" {
			t.Errorf("header = %q, want %q", lines[0], "name\tage")
		}
		if lines[1] != "Alice\t30" {
			t.Errorf("row 1 = %q, want %q", lines[1], "Alice\t30")
		}
	})

	t.Run("tsv custom delimiter", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "tsv", Delimiter: "|"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if lines[0] != "name|age" {
			t.Errorf("header = %q, want %q", lines[0], "name|age")
		}
		if lines[1] != "Alice|30" {
			t.Errorf("row 1 = %q, want %q", lines[1], "Alice|30")
		}
	})
}

func TestFormatNDJSON(t *testing.T) {
	t.Run("one json object per line", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(sampleCols, sampleRows)
		if err := Format(&buf, resp, Options{Format: "ndjson"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines (one per row), got %d", len(lines))
		}

		for i, line := range lines {
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				t.Fatalf("line %d: invalid JSON: %v", i, err)
			}
			if _, ok := obj["name"]; !ok {
				t.Errorf("line %d missing 'name' key", i)
			}
		}
	})
}

func TestFormatUnknown(t *testing.T) {
	var buf bytes.Buffer
	resp := makeResp(sampleCols, sampleRows)
	err := Format(&buf, resp, Options{Format: "xml"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("error %q should mention 'unknown format'", err)
	}
}

// ---------- Test 4: Column filtering ----------

func TestFormatWithColumnFilter(t *testing.T) {
	threeCols := []client.Column{
		{Name: "name", Type: "keyword"},
		{Name: "age", Type: "integer"},
		{Name: "city", Type: "keyword"},
	}
	threeColRows := [][]interface{}{
		{"Alice", float64(30), "NYC"},
		{"Bob", float64(25), "SF"},
	}

	t.Run("select subset of columns", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(threeCols, threeColRows)
		err := Format(&buf, resp, Options{Format: "csv", Columns: []string{"name", "city"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if lines[0] != "name,city" {
			t.Errorf("header = %q, want %q", lines[0], "name,city")
		}
		if lines[1] != "Alice,NYC" {
			t.Errorf("row 1 = %q, want %q", lines[1], "Alice,NYC")
		}
	})

	t.Run("nonexistent column returns all columns", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(threeCols, threeColRows)
		err := Format(&buf, resp, Options{Format: "csv", Columns: []string{"nonexistent"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if lines[0] != "name,age,city" {
			t.Errorf("header = %q, want all columns when filter matches nothing", lines[0])
		}
	})

	t.Run("select single column", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(threeCols, threeColRows)
		err := Format(&buf, resp, Options{Format: "csv", Columns: []string{"age"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if lines[0] != "age" {
			t.Errorf("header = %q, want %q", lines[0], "age")
		}
		if lines[1] != "30" {
			t.Errorf("row 1 = %q, want %q", lines[1], "30")
		}
	})

	t.Run("column order matches response order", func(t *testing.T) {
		var buf bytes.Buffer
		resp := makeResp(threeCols, threeColRows)
		// request city before name, but response has name before city
		err := Format(&buf, resp, Options{Format: "csv", Columns: []string{"city", "name"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		// filterColumns preserves response column order, not request order
		if lines[0] != "name,city" {
			t.Errorf("header = %q, want %q (response order)", lines[0], "name,city")
		}
	})
}

func TestFullResponseOutput(t *testing.T) {
	var response client.QueryResponse
	raw := []byte(`{"id":"pending-id","is_running":true,"is_partial":true,"cpu_nanos":9007199254740993,"profile":{"driver":"detail"}}`)
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Format(&buf, &response, Options{Format: "json", Raw: true}); err != nil {
		t.Fatal(err)
	}
	var got, want interface{}
	decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&got); err != nil {
		t.Fatal(err)
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("output=%s", buf.String())
	}
	if err := Format(&buf, &response, Options{Format: "csv"}); err == nil {
		t.Fatal("formatted a pending query as completed rows")
	}
}
