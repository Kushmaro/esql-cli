package formatter

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/parquet-go/parquet-go"
)

func TestParquetRoundTrip(t *testing.T) {
	resp := &client.QueryResponse{
		Columns: []client.Column{{Name: "name", Type: "keyword"}, {Name: "count", Type: "long"}, {Name: "active", Type: "boolean"}},
		Values:  [][]interface{}{{"hello", float64(42), true}, {"", float64(0), false}, {nil, nil, nil}},
	}
	var buf bytes.Buffer
	if err := Format(&buf, resp, Options{Format: "parquet"}); err != nil {
		t.Fatal(err)
	}
	rows, err := parquet.Read[any](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"name": "hello", "count": int64(42), "active": true},
		map[string]any{"name": "", "count": int64(0), "active": false},
		map[string]any{"name": nil, "count": nil, "active": nil},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
}

func TestParquetExactIntegers(t *testing.T) {
	resp := &client.QueryResponse{
		Columns: []client.Column{{Name: "signed", Type: "long"}, {Name: "unsigned", Type: "unsigned_long"}},
		Values:  [][]interface{}{{json.Number("9223372036854775807"), json.Number("18446744073709551615")}},
	}
	var buf bytes.Buffer
	if err := Format(&buf, resp, Options{Format: "parquet"}); err != nil {
		t.Fatal(err)
	}
	type numbers struct {
		Signed   int64  `parquet:"signed"`
		Unsigned uint64 `parquet:"unsigned"`
	}
	rows, err := parquet.Read[numbers](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := []numbers{{Signed: 9223372036854775807, Unsigned: 18446744073709551615}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
}

func TestParquetLists(t *testing.T) {
	resp := &client.QueryResponse{
		Columns: []client.Column{{Name: "tags", Type: "keyword"}, {Name: "numbers", Type: "long"}},
		Values: [][]interface{}{
			{[]interface{}{"a", "b"}, []interface{}{json.Number("9007199254740993"), nil, json.Number("0")}},
			{"single", json.Number("7")},
			{[]interface{}{}, []interface{}{}},
			{nil, nil},
		},
	}
	var buf bytes.Buffer
	if err := Format(&buf, resp, Options{Format: "parquet"}); err != nil {
		t.Fatal(err)
	}
	rows, err := parquet.Read[any](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"tags": []any{"a", "b"}, "numbers": []any{int64(9007199254740993), nil, int64(0)}},
		map[string]any{"tags": []any{"single"}, "numbers": []any{int64(7)}},
		map[string]any{"tags": []any{}, "numbers": []any{}},
		map[string]any{"tags": nil, "numbers": nil},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
}

func TestParquetSchemaAndEmptyResults(t *testing.T) {
	for _, empty := range []bool{false, true} {
		resp := &client.QueryResponse{
			Columns: []client.Column{{Name: "z", Type: "date"}, {Name: "a", Type: "date_nanos"}, {Name: "host.name", Type: "keyword"}},
		}
		if !empty {
			resp.Values = [][]interface{}{{"2026-10-18T00:00:00.123Z", "2026-10-18T00:00:00.123456789Z", "server"}}
		}
		var buf bytes.Buffer
		if err := Format(&buf, resp, Options{Format: "parquet"}); err != nil {
			t.Fatal(err)
		}
		file, err := parquet.OpenFile(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		for i, field := range file.Schema().Fields() {
			if field.Name() != resp.Columns[i].Name {
				t.Fatalf("wrong field order: %s", file.Schema())
			}
		}
		if empty && file.NumRows() != 0 {
			t.Fatalf("empty export has %d rows", file.NumRows())
		}
		if !empty {
			type timestamps struct {
				Z    int64  `parquet:"z"`
				A    int64  `parquet:"a"`
				Host string `parquet:"host.name"`
			}
			rows, err := parquet.Read[timestamps](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatal(err)
			}
			want := []timestamps{{Z: 1792281600123, A: 1792281600123456789, Host: "server"}}
			if !reflect.DeepEqual(rows, want) {
				t.Fatalf("timestamps = %#v", rows)
			}
		}
	}
}

func TestParquetInvalidData(t *testing.T) {
	tests := []struct {
		name string
		resp client.QueryResponse
	}{
		{"no columns", client.QueryResponse{}},
		{"partial response", client.QueryResponse{IsPartial: true, Columns: []client.Column{{Name: "a"}}}},
		{"duplicate columns", client.QueryResponse{Columns: []client.Column{{Name: "a"}, {Name: "a"}}}},
		{"short row", client.QueryResponse{Columns: []client.Column{{Name: "a"}}, Values: [][]interface{}{{}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Format(&buf, &tt.resp, Options{Format: "parquet"}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	for _, tt := range []struct {
		typ string
		val any
	}{
		{"long", json.Number("9223372036854775808")}, {"unsigned_long", json.Number("-1")},
		{"integer", json.Number("2147483648")}, {"integer", 1.5}, {"boolean", "false"},
		{"double", "bad"}, {"date", "bad"}, {"date_nanos", "2500-01-01T00:00:00Z"},
		{"long", []any{json.Number("1"), "bad"}},
	} {
		t.Run(tt.typ+"/"+formatValue(tt.val), func(t *testing.T) {
			resp := &client.QueryResponse{Columns: []client.Column{{Name: "a", Type: tt.typ}}, Values: [][]interface{}{{tt.val}}}
			var buf bytes.Buffer
			if err := Format(&buf, resp, Options{Format: "parquet"}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParquetColumnSelection(t *testing.T) {
	resp := &client.QueryResponse{
		Columns: []client.Column{{Name: "drop", Type: "keyword"}, {Name: "keep", Type: "integer"}},
		Values:  [][]interface{}{{"hidden", json.Number("42")}},
	}
	var buf bytes.Buffer
	if err := Format(&buf, resp, Options{Format: "parquet", Columns: []string{"keep"}}); err != nil {
		t.Fatal(err)
	}
	rows, err := parquet.Read[any](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, []any{map[string]any{"keep": int32(42)}}) {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestParquetMalformedRowWithColumnSelection(t *testing.T) {
	resp := &client.QueryResponse{
		Columns: []client.Column{{Name: "missing", Type: "long"}},
		Values:  [][]interface{}{{}},
	}
	var buf bytes.Buffer
	if err := Format(&buf, resp, Options{Format: "parquet", Columns: []string{"missing"}}); err == nil {
		t.Fatal("expected a malformed-row error before column selection")
	}
}

func TestParquetWriteFailure(t *testing.T) {
	resp := &client.QueryResponse{Columns: []client.Column{{Name: "a", Type: "keyword"}}, Values: [][]interface{}{{"value"}}}
	if err := Format(failingParquetWriter{}, resp, Options{Format: "parquet"}); err == nil {
		t.Fatal("expected write error")
	}
}

type failingParquetWriter struct{}

func (failingParquetWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
