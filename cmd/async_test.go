package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func useTestCluster(t *testing.T, host string) {
	t.Helper()
	for _, entry := range []struct {
		key   string
		value string
	}{{"host", host}, {"api_key", "test-key"}, {"cluster", ""}, {"default_cluster", ""}} {
		key, old := entry.key, viper.Get(entry.key)
		viper.Set(key, entry.value)
		t.Cleanup(func() { viper.Set(key, old) })
	}
}

func runTestCommand(t *testing.T, cmd *cobra.Command, flags, args []string) error {
	t.Helper()
	cmd.SetContext(context.Background())
	if err := cmd.ParseFlags(flags); err != nil {
		return err
	}
	return cmd.RunE(cmd, args)
}

func TestAsyncCommands(t *testing.T) {
	var routes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes = append(routes, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "ApiKey test-key" {
			t.Error("missing auth")
		}
		if r.URL.Path == "/_query/async" {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["query"] != "ROW n = ?n" || body["wait_for_completion_timeout"] != "0s" || body["keep_on_completion"] != true {
				t.Errorf("body=%v", body)
			}
			if body["params"] == nil || body["settings"] == nil {
				t.Error("missing native options")
			}
			fmt.Fprint(w, `{"id":"test-id","is_running":true,"is_partial":true}`)
		} else if r.Method == "DELETE" {
			fmt.Fprint(w, `{"acknowledged":true}`)
		} else {
			fmt.Fprint(w, `{"id":"test-id","is_running":false,"columns":[{"name":"n","type":"long"}],"values":[[42]],"cpu_nanos":123}`)
		}
	}))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	dir := t.TempDir()
	for _, op := range []string{"submit", "get", "stop", "delete"} {
		cmd, _, err := newAsyncCommand().Find([]string{op})
		if err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(dir, op+".json")
		flags := []string{"--output", output}
		args := []string{"test-id"}
		if op == "submit" {
			args = []string{"ROW n = ?n"}
			flags = append(flags, "--bind", "n=42", "--setting", `time_zone="UTC"`, "--wait-for-completion", "0s")
		}
		if err := runTestCommand(t, cmd, flags, args); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if op == "submit" && result["id"] != "test-id" {
			t.Fatal("pending submission lost its ID")
		}
		if op == "get" && result["cpu_nanos"] != float64(123) {
			t.Fatal("lost metadata in default async output")
		}
	}
	expected := []string{"POST /_query/async", "GET /_query/async/test-id", "POST /_query/async/test-id/stop", "DELETE /_query/async/test-id"}
	if strings.Join(routes, ",") != strings.Join(expected, ",") {
		t.Fatalf("routes=%v", routes)
	}
}

func TestPendingAsyncDoesNotOverwriteParquet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"id":"test-id","is_running":true}`) }))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	output := filepath.Join(t.TempDir(), "existing.parquet")
	os.WriteFile(output, []byte("existing"), 0600)
	cmd, _, _ := newAsyncCommand().Find([]string{"get"})
	err := runTestCommand(t, cmd, []string{"--format", "parquet", "-o", output}, []string{"test-id"})
	if err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("error=%v", err)
	}
	data, _ := os.ReadFile(output)
	if string(data) != "existing" {
		t.Fatal("overwrote existing file")
	}
}

func TestRunningCommands(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_query/queries" {
			fmt.Fprint(w, `{"queries":{}}`)
		} else if r.URL.Path == "/_query/queries/node:1" {
			fmt.Fprint(w, `{"id":1,"data_nodes":["node"]}`)
		} else {
			t.Error(r.URL.Path)
		}
	}))
	defer srv.Close()
	useTestCluster(t, srv.URL)
	for _, args := range [][]string{nil, {"node:1"}} {
		output := filepath.Join(t.TempDir(), "response.json")
		if err := runTestCommand(t, newRunningCommand(), []string{"-o", output}, args); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(output)
		if !json.Valid(data) {
			t.Fatalf("invalid output: %s", data)
		}
	}
}
