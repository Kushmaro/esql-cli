package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPromQLCommands(t *testing.T) {
	var path, method string
	var params url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		params = r.Form
		if r.Header.Get("Authorization") != "ApiKey test-key" {
			t.Error("missing authentication")
		}
		fmt.Fprint(w, `{"status":"success","data":[],"warnings":["test warning"],"extra":9007199254740993}`)
	}))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	tests := []struct {
		op           string
		args, flags  []string
		path, method string
		params       url.Values
	}{
		{op: "query", args: []string{`vector(1)`}, flags: []string{"--time", "1792281600.123456789", "--limit", "0", "--query-timeout", "30s"}, path: "query", method: "GET", params: url.Values{"query": {"vector(1)"}, "time": {"1792281600.123456789"}, "limit": {"0"}, "timeout": {"30s"}}},
		{op: "range", args: []string{"vector(2)"}, flags: []string{"--start", "0", "--end", "60", "--step", "30s", "--method", "POST"}, path: "query_range", method: "POST", params: url.Values{"query": {"vector(2)"}, "start": {"0"}, "end": {"60"}, "step": {"30s"}}},
		{op: "labels", flags: []string{"--match", `a{job="x,y"}`, "--match", "b", "--start", "0", "--end", "1"}, path: "labels", method: "GET", params: url.Values{"match[]": {`a{job="x,y"}`, "b"}, "start": {"0"}, "end": {"1"}}},
		{op: "label-values", args: []string{"__name__"}, path: "label/__name__/values", method: "GET", params: url.Values{}},
		{op: "series", flags: []string{"--match", "up"}, path: "series", method: "GET", params: url.Values{"match[]": {"up"}}},
		{op: "metadata", flags: []string{"--metric", "up", "--limit-per-metric", "2"}, path: "metadata", method: "GET", params: url.Values{"metric": {"up"}, "limit_per_metric": {"2"}}},
		{op: "buildinfo", path: "status/buildinfo", method: "GET", params: url.Values{}},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			cmd, _, err := newPromQLCommand().Find([]string{tt.op})
			if err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "response.json")
			flags := append(append([]string{}, tt.flags...), "--index", "metrics-*", "-o", output)
			if err := runTestCommand(t, cmd, flags, tt.args); err != nil {
				t.Fatal(err)
			}
			if path != "/_prometheus/metrics-*/api/v1/"+tt.path || method != tt.method || !reflect.DeepEqual(params, tt.params) {
				t.Fatalf("%s %s %v", method, path, params)
			}
			data, err := os.ReadFile(output)
			if err != nil || !strings.Contains(string(data), "test warning") || !strings.Contains(string(data), "9007199254740993") {
				t.Fatalf("lost envelope data %s %v", data, err)
			}
		})
	}
}

func TestPromQLInvalidOptions(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; fmt.Fprint(w, `{"status":"success"}`) }))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	tests := []struct {
		op          string
		flags, args []string
	}{
		{op: "query", args: []string{" "}},
		{op: "range", args: []string{"vector(1)"}, flags: []string{"--start", "0", "--end", "1"}},
		{op: "series"}, {op: "labels", flags: []string{"--match", ""}},
		{op: "query", args: []string{"vector(1)"}, flags: []string{"--limit", "-1"}},
		{op: "metadata", flags: []string{"--limit-per-metric", "-1"}},
		{op: "query", args: []string{"vector(1)"}, flags: []string{"--method", "DELETE"}},
		{op: "metadata", flags: []string{"--method", "POST"}},
	}
	for _, tt := range tests {
		cmd, _, _ := newPromQLCommand().Find([]string{tt.op})
		if err := runTestCommand(t, cmd, tt.flags, tt.args); err == nil {
			t.Fatalf("accepted %+v", tt)
		}
	}
	if requests != 0 {
		t.Fatalf("invalid options made %d requests", requests)
	}
}
