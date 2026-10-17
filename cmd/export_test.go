package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/exporter"
	"github.com/parquet-go/parquet-go"
)

func TestCompleteExportToFile(t *testing.T) {
	searches, closed := 0, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE":
			closed = true
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/logs/_pit":
			fmt.Fprint(w, `{"id":"pit"}`)
		default:
			searches++
			if searches == 1 {
				fmt.Fprint(w, `{"hits":{"hits":[{"_index":"logs","_id":"a","_source":{"n":9007199254740993},"sort":[1]}]}}`)
			} else {
				fmt.Fprint(w, `{"hits":{"hits":[]}}`)
			}
		}
	}))
	defer srv.Close()
	output := filepath.Join(t.TempDir(), "export.parquet")
	err := writeResults(output, "parquet", func(w io.Writer) error {
		_, err := exporter.Export(context.Background(), client.New(client.Config{Host: srv.URL}), "logs", 1, w)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parquet.Read[any](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].(map[string]any)["n"] != int64(9007199254740993) || !closed {
		t.Fatalf("rows=%#v closed=%v", rows, closed)
	}
}

func TestParquetOutputIsAtomic(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "existing.parquet")
	original := []byte("existing content")
	if err := os.WriteFile(output, original, 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("failed query")
	err := writeResults(output, "parquet", func(w io.Writer) error { w.Write([]byte("incomplete data")); return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("existing file modified: %q %v", data, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files leaked: %v %v", files, err)
	}
}

func TestParquetRequiresOneQuery(t *testing.T) {
	for _, n := range []int{0, 2} {
		if err := validateParquetOutput("PARQUET", "file.parquet", n); err == nil {
			t.Fatalf("accepted %d queries", n)
		}
	}
	if err := validateParquetOutput("parquet", "file.parquet", 1); err != nil {
		t.Fatal(err)
	}
}
