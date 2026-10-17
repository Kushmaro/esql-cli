package exporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/parquet-go/parquet-go"
)

type fakeScanner struct {
	docs []client.Document
	err  error
}

func (f fakeScanner) Scan(ctx context.Context, _ string, _ int, visit func(client.Document) error) error {
	for _, doc := range f.docs {
		if err := visit(doc); err != nil {
			return err
		}
	}
	return f.err
}

func TestExportSchemaDiscovery(t *testing.T) {
	docs := []client.Document{
		{Index: "logs-1", ID: "1", Source: map[string]interface{}{
			"name": "first", "n": json.Number("9007199254740993"), "mixed": true, "tags": "one", "object": map[string]interface{}{"a": json.Number("1")},
		}},
		{Index: "logs-2", ID: "2", Source: map[string]interface{}{
			"name": "second", "n": json.Number("2"), "mixed": "true", "tags": []interface{}{"two", nil}, "late": false, "object": map[string]interface{}{"b": "new"},
		}},
		{Index: "logs-2", ID: "3", Source: map[string]interface{}{"tags": []interface{}{}, "mixed": json.Number("1.5")}},
	}
	var buf bytes.Buffer
	n, err := Export(context.Background(), fakeScanner{docs: docs}, "logs-*", 1000, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("count=%d", n)
	}
	rows, err := parquet.Read[any](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"_index": "logs-1", "_id": "1", "name": "first", "n": int64(9007199254740993), "mixed": "true", "tags": []any{"one"}, "late": nil, "object": `{"a":1}`},
		map[string]any{"_index": "logs-2", "_id": "2", "name": "second", "n": int64(2), "mixed": `"true"`, "tags": []any{"two", nil}, "late": false, "object": `{"b":"new"}`},
		map[string]any{"_index": "logs-2", "_id": "3", "name": nil, "n": nil, "mixed": "1.5", "tags": []any{}, "late": nil, "object": nil},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows=%#v, want=%#v", rows, want)
	}
}

func TestExportEmpty(t *testing.T) {
	var buf bytes.Buffer
	n, err := Export(context.Background(), fakeScanner{}, "empty", 1, &buf)
	if err != nil || n != 0 {
		t.Fatalf("count=%d error=%v", n, err)
	}
	file, err := parquet.OpenFile(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if file.NumRows() != 0 || len(file.Schema().Fields()) != 2 {
		t.Fatal("invalid empty export")
	}
}

func TestExportFailures(t *testing.T) {
	sentinel := errors.New("search failed")
	var buf bytes.Buffer
	if _, err := Export(context.Background(), fakeScanner{err: sentinel}, "logs", 1, &buf); !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
	if buf.Len() != 0 {
		t.Fatal("wrote data after failed scan")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Export(ctx, fakeScanner{}, "logs", 1, &buf); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	docs := []client.Document{{Index: "logs", Source: map[string]interface{}{"_id": "collision"}}}
	if _, err := Export(context.Background(), fakeScanner{docs: docs}, "logs", 1, &buf); err == nil {
		t.Fatal("expected metadata collision error")
	}
	if _, err := Export(context.Background(), fakeScanner{}, "empty", 1, failWriter{}); err == nil {
		t.Fatal("expected write error")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
