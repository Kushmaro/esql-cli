package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func nativeCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	addNativeQueryFlags(cmd)
	addResultFlags(cmd)
	return cmd
}

func TestNativeFlags(t *testing.T) {
	cmd := nativeCommand()
	query := `ROW n = ?n, message = ?message`
	payload := `" | DROP *; ROW injected = true`
	encoded, _ := json.Marshal(payload)
	if err := cmd.ParseFlags([]string{"--bind", "n=9223372036854775807", "--bind", "message=" + string(encoded), "--setting", `time_zone="Europe/Amsterdam"`, "--setting", "approximation=false", "--time-zone", "UTC", "--profile", "--execution-metadata"}); err != nil {
		t.Fatal(err)
	}
	req, err := queryRequest(cmd, query)
	if err != nil {
		t.Fatal(err)
	}
	if req.Query != query || req.Params[0].(map[string]interface{})["n"] != json.Number("9223372036854775807") || req.Params[1].(map[string]interface{})["message"] != payload {
		t.Fatalf("request=%+v", req)
	}
	if req.Settings["time_zone"] != "Europe/Amsterdam" || req.Settings["approximation"] != false || req.TimeZone != "UTC" || !req.Profile || !req.IncludeExecutionMetadata {
		t.Fatalf("options=%+v", req)
	}
}

func TestJSONOptionFilesAndTypes(t *testing.T) {
	dir := t.TempDir()
	params := filepath.Join(dir, "params.json")
	if err := os.WriteFile(params, []byte(`[9223372036854775807,true,null,[1,2],"a;b"]`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := nativeCommand()
	if err := cmd.ParseFlags([]string{"--params", "@" + params, "--settings", `{"approximation":{"rows":10000},"column_metadata":true}`}); err != nil {
		t.Fatal(err)
	}
	req, err := queryRequest(cmd, "ROW n = ?")
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Params) != 5 || req.Params[0] != json.Number("9223372036854775807") || req.Params[1] != true || req.Params[2] != nil {
		t.Fatalf("params=%v", req.Params)
	}
	if req.Settings["approximation"].(map[string]interface{})["rows"] != json.Number("10000") {
		t.Fatalf("settings=%v", req.Settings)
	}
}

func TestInvalidNativeFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--params", "null"}, {"--params", `{"n":1}`}, {"--params", "[] true"},
		{"--bind", "missing-equals"}, {"--bind", "=1"}, {"--bind", "n=unquoted string"},
		{"--bind", "n=1", "--bind", "n=2"}, {"--params", "[1]", "--bind", "n=2"},
		{"--settings", "[]"}, {"--settings", "null"}, {"--setting", `x={broken`},
		{"--settings", `{"x":1}`, "--setting", "x=2"}, {"--setting", "x=true", "--setting", "x=false"},
		{"--params", "@/nonexistent/native-params.json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := nativeCommand()
			if err := cmd.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if _, err := queryRequest(cmd, "ROW n = ?n"); err == nil {
				t.Fatalf("accepted invalid options: %v", args)
			}
		})
	}
}

func TestResponseOptions(t *testing.T) {
	for _, tt := range []struct {
		args    []string
		async   bool
		raw     bool
		format  string
		invalid bool
	}{
		{args: []string{"--raw"}, raw: true, format: "json"},
		{async: true, raw: true, format: "json"},
		{args: []string{"--format", "parquet"}, async: true, format: "parquet"},
		{args: []string{"--format", "json"}, async: true, format: "json"},
		{args: []string{"--raw", "--format", "csv"}, invalid: true},
		{args: []string{"--raw", "--columns", "n"}, invalid: true},
		{args: []string{"--format", "bad"}, invalid: true},
	} {
		cmd := nativeCommand()
		if err := cmd.ParseFlags(tt.args); err != nil {
			t.Fatal(err)
		}
		opts, err := resultOptions(cmd, tt.async)
		if tt.invalid {
			if err == nil {
				t.Fatalf("accepted %v", tt.args)
			}
			continue
		}
		if err != nil || opts.Raw != tt.raw || opts.Format != tt.format {
			t.Fatalf("options=%+v error=%v", opts, err)
		}
	}
}
