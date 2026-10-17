package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestViewCommands(t *testing.T) {
	var routes []string
	definition := `ROW n = 9007199254740993, message = "a;b"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes = append(routes, r.Method+" "+r.URL.Path)
		switch r.Method {
		case "PUT":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["query"] != definition {
				t.Errorf("definition changed: %v", body)
			}
			if _, ok := body["description"]; ok {
				t.Error("description sent without explicit flag")
			}
			fmt.Fprint(w, `{"acknowledged":true}`)
		case "GET":
			fmt.Fprint(w, `{"views":[{"name":"test","query":"ROW n=1","extra":9007199254740993}]}`)
		case "DELETE":
			fmt.Fprint(w, `{"acknowledged":true}`)
		default:
			t.Error(r.Method)
		}
	}))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	dir := t.TempDir()
	source := filepath.Join(dir, "query.esql")
	if err := os.WriteFile(source, []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"list", "put", "get", "delete"} {
		cmd, _, err := newViewsCommand().Find([]string{op})
		if err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(dir, op+".json")
		args := []string{"a", "b"}
		flags := []string{"-o", output}
		if op == "list" {
			args = nil
		} else if op == "put" {
			args = []string{"test"}
			flags = append(flags, "--file", source)
		}
		if err := runTestCommand(t, cmd, flags, args); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		data, err := os.ReadFile(output)
		if err != nil || !json.Valid(data) {
			t.Fatalf("invalid response %s %v", data, err)
		}
		if op == "get" && !strings.Contains(string(data), "9007199254740993") {
			t.Fatal("metadata precision lost")
		}
	}
	want := []string{"GET /_query/view", "PUT /_query/view/test", "GET /_query/view/a,b", "DELETE /_query/view/a,b"}
	if strings.Join(routes, ",") != strings.Join(want, ",") {
		t.Fatalf("routes=%v", routes)
	}
}

func TestSingleViewQueryInput(t *testing.T) {
	query := `SET time_zone="UTC"; ROW message="a;b"`
	for _, mode := range []string{"argument", "stdin", "file"} {
		t.Run(mode, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("file", "", "")
			args := []string{query}
			switch mode {
			case "stdin":
				args = []string{"-"}
				cmd.SetIn(strings.NewReader(query))
			case "file":
				args = nil
				path := filepath.Join(t.TempDir(), "view.esql")
				os.WriteFile(path, []byte(query), 0600)
				cmd.Flags().Set("file", path)
			}
			got, err := readSingleQuery(cmd, args)
			if err != nil || got != query {
				t.Fatalf("query=%q err=%v", got, err)
			}
		})
	}
	for _, args := range [][]string{nil, {" "}, {"a", "b"}} {
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		if _, err := readSingleQuery(cmd, args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("file", "", " ")
	cmd.Flags().Set("file", "/nonexistent/file")
	if _, err := readSingleQuery(cmd, []string{"ROW n=1"}); err == nil {
		t.Fatal("accepted conflicting inputs")
	}
	if _, err := readSingleQuery(cmd, nil); err == nil {
		t.Fatal("accepted missing file")
	}
}
