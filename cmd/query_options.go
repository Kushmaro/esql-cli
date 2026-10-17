package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

func addNativeQueryFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringArray("bind", nil, `native named parameter name=JSON (repeatable; e.g. n=42 or 'name="Ada"')`)
	f.String("params", "", "native parameter array as JSON or @file (positional, named, or typed bindings)")
	f.StringArray("setting", nil, `query setting name=JSON (repeatable; e.g. 'time_zone="Europe/Amsterdam"')`)
	f.String("settings", "", "query settings object as JSON or @file")
	f.String("time-zone", "", "default query time zone")
	f.String("locale", "", "query locale")
	f.Bool("profile", false, "request query profiling (use --raw to display)")
	f.Bool("execution-metadata", false, "request execution metadata (use --raw to display)")
	cmd.MarkFlagsMutuallyExclusive("bind", "params")
}

func queryRequest(cmd *cobra.Command, query string) (client.QueryRequest, error) {
	req := client.QueryRequest{Query: query}
	params, _ := cmd.Flags().GetString("params")
	if cmd.Flags().Changed("params") {
		if err := decodeJSONFlag("params", params, &req.Params); err != nil {
			return req, err
		}
		if req.Params == nil {
			return req, fmt.Errorf("--params must be a JSON array")
		}
	}
	binds, _ := cmd.Flags().GetStringArray("bind")
	if len(binds) > 0 && cmd.Flags().Changed("params") {
		return req, fmt.Errorf("--bind and --params cannot be combined")
	}
	seen := map[string]bool{}
	for _, binding := range binds {
		name, value, err := jsonAssignment("bind", binding)
		if err != nil {
			return req, err
		}
		if seen[name] {
			return req, fmt.Errorf("duplicate native parameter %q", name)
		}
		seen[name] = true
		req.Params = append(req.Params, map[string]interface{}{name: value})
	}
	settings, _ := cmd.Flags().GetString("settings")
	if cmd.Flags().Changed("settings") {
		if err := decodeJSONFlag("settings", settings, &req.Settings); err != nil {
			return req, err
		}
		if req.Settings == nil {
			return req, fmt.Errorf("--settings must be a JSON object")
		}
	}
	assignments, _ := cmd.Flags().GetStringArray("setting")
	for _, assignment := range assignments {
		name, value, err := jsonAssignment("setting", assignment)
		if err != nil {
			return req, err
		}
		if req.Settings == nil {
			req.Settings = make(map[string]interface{})
		}
		if _, exists := req.Settings[name]; exists {
			return req, fmt.Errorf("duplicate query setting %q", name)
		}
		req.Settings[name] = value
	}
	req.TimeZone, _ = cmd.Flags().GetString("time-zone")
	req.Locale, _ = cmd.Flags().GetString("locale")
	req.Profile, _ = cmd.Flags().GetBool("profile")
	req.IncludeExecutionMetadata, _ = cmd.Flags().GetBool("execution-metadata")
	return req, nil
}

func jsonAssignment(flag, value string) (string, interface{}, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", nil, fmt.Errorf("invalid --%s %q (expected name=JSON)", flag, value)
	}
	var decoded interface{}
	if err := decodeJSONFlag(flag, parts[1], &decoded); err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(parts[0]), decoded, nil
}

func decodeJSONFlag(flag, value string, target interface{}) error {
	if strings.HasPrefix(value, "@") {
		data, err := os.ReadFile(value[1:])
		if err != nil {
			return fmt.Errorf("reading --%s file: %w", flag, err)
		}
		value = string(data)
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid --%s JSON: %w", flag, err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("--%s must contain exactly one JSON value", flag)
	}
	return nil
}

func addResultFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringP("format", "f", "", "output format: table, json, ndjson, csv, tsv, parquet")
	f.StringP("output", "o", "", "write output to a file")
	f.Bool("no-header", false, "suppress column headers")
	f.Bool("quiet", false, "suppress timing and row counts")
	f.String("columns", "", "comma-separated output columns")
	f.String("delimiter", "", "custom field delimiter (implies tsv)")
	f.Bool("raw", false, "return the full JSON response including metadata")
}

func resultOptions(cmd *cobra.Command, async bool) (formatter.Options, error) {
	f := cmd.Flags()
	format, _ := f.GetString("format")
	raw, _ := f.GetBool("raw")
	if async && !f.Changed("format") && !f.Changed("raw") {
		raw = true
	}
	delimiter, _ := f.GetString("delimiter")
	if !f.Changed("format") {
		if raw {
			format = "json"
		} else if delimiter != "" {
			format = "tsv"
		}
	}
	if format == "" {
		if raw {
			format = "json"
		} else if delimiter != "" {
			format = "tsv"
		} else {
			format = formatter.DefaultFormat()
		}
	}
	columns, _ := f.GetString("columns")
	if raw && (!strings.EqualFold(format, "json") || columns != "" || delimiter != "") {
		return formatter.Options{}, fmt.Errorf("full-response output requires JSON without --columns or --delimiter; specify --format for result rows")
	}
	noHeader, _ := f.GetBool("no-header")
	quiet, _ := f.GetBool("quiet")
	opts := formatter.Options{Format: format, Raw: raw, NoHeader: noHeader, Quiet: quiet, Delimiter: delimiter}
	for _, col := range strings.Split(columns, ",") {
		if col = strings.TrimSpace(col); col != "" {
			opts.Columns = append(opts.Columns, col)
		}
	}
	switch strings.ToLower(format) {
	case "table", "json", "ndjson", "csv", "tsv", "parquet":
	default:
		return opts, fmt.Errorf("unknown format %q", format)
	}
	return opts, nil
}
