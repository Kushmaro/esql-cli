package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/spf13/cobra"
)

func newPromQLCommand() *cobra.Command {
	parent := &cobra.Command{Use: "promql", Short: "Query Elasticsearch through its Prometheus-compatible HTTP API", Long: `Run PromQL instant/range queries and discover labels, series and metric metadata.
Responses retain the full Prometheus JSON envelope, including warnings and sample
strings. --index scopes the request; otherwise the server defaults to metrics-*.
GET is the default. POST uses form encoding on endpoints that support it;
the deployment must accept authenticated form requests over Elasticsearch HTTP TLS.`}
	addAPIPersistentFlags(parent)
	parent.PersistentFlags().String("index", "", "index expression or alias to scope PromQL requests")
	parent.PersistentFlags().String("method", "GET", "HTTP method: GET or POST (where supported)")
	type operation struct {
		name, endpoint, short            string
		expression, timeRange, discovery bool
		args                             cobra.PositionalArgs
	}
	operations := []operation{
		{name: "query", endpoint: "query", short: "Evaluate an instant PromQL expression", expression: true, args: cobra.ExactArgs(1)},
		{name: "range", endpoint: "query_range", short: "Evaluate a PromQL expression over a time range", expression: true, timeRange: true, args: cobra.ExactArgs(1)},
		{name: "labels", endpoint: "labels", short: "List label names", discovery: true, args: cobra.NoArgs},
		{name: "label-values", endpoint: "label_values", short: "List values for one label", discovery: true, args: cobra.ExactArgs(1)},
		{name: "series", endpoint: "series", short: "Find series matching one or more selectors", discovery: true, args: cobra.NoArgs},
		{name: "metadata", endpoint: "metadata", short: "Get metric type, help and unit metadata", args: cobra.NoArgs},
		{name: "buildinfo", endpoint: "buildinfo", short: "Get Prometheus-compatible build information", args: cobra.NoArgs},
	}
	for _, op := range operations {
		op := op
		use := op.name
		if op.expression {
			use += " <expression>"
		} else if op.name == "label-values" {
			use += " <label>"
		}
		cmd := &cobra.Command{Use: use, Short: op.short, Args: op.args, SilenceUsage: true}
		if op.expression {
			cmd.Flags().Int("limit", 0, "maximum number of returned series (0 uses the server limit)")
			cmd.Flags().String("query-timeout", "", "server query timeout, e.g. 30s (separate from HTTP --timeout)")
			if op.timeRange {
				cmd.Flags().String("start", "", "range start as RFC3339 or Unix seconds (required)")
				cmd.Flags().String("end", "", "range end as RFC3339 or Unix seconds (required)")
				cmd.Flags().String("step", "", "range step as a duration or seconds (required)")
			} else {
				cmd.Flags().String("time", "", "evaluation time as RFC3339 or Unix seconds")
			}
		}
		if op.discovery {
			cmd.Flags().StringArray("match", nil, "series selector (repeatable; sent as match[])")
			cmd.Flags().String("start", "", "range start as RFC3339 or Unix seconds")
			cmd.Flags().String("end", "", "range end as RFC3339 or Unix seconds")
			cmd.Flags().Int("limit", 0, "maximum number of results (0 uses the server limit)")
		}
		if op.name == "metadata" {
			cmd.Flags().String("metric", "", "metric name to inspect")
			cmd.Flags().Int("limit", 0, "maximum number of metric names (0 uses the server limit)")
			cmd.Flags().Int("limit-per-metric", 0, "maximum metadata entries per metric")
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			request := client.PromQLRequest{Endpoint: op.endpoint, Params: params}
			request.Index, _ = cmd.Flags().GetString("index")
			request.Method, _ = cmd.Flags().GetString("method")
			if op.expression {
				if strings.TrimSpace(args[0]) == "" {
					return fmt.Errorf("PromQL expression cannot be empty")
				}
				params.Set("query", args[0])
			}
			if op.name == "label-values" {
				if args[0] == "" {
					return fmt.Errorf("label name is required")
				}
				request.Label = args[0]
			}
			for _, flag := range []string{"time", "start", "end", "step", "metric", "query-timeout"} {
				if cmd.Flags().Lookup(flag) == nil {
					continue
				}
				value, _ := cmd.Flags().GetString(flag)
				if op.timeRange && (flag == "start" || flag == "end" || flag == "step") && value == "" {
					return fmt.Errorf("--%s is required for range queries", flag)
				}
				if value != "" {
					key := flag
					if key == "query-timeout" {
						key = "timeout"
					}
					params.Set(key, value)
				}
			}
			for _, flag := range []string{"limit", "limit-per-metric"} {
				if cmd.Flags().Lookup(flag) == nil {
					continue
				}
				value, _ := cmd.Flags().GetInt(flag)
				if value < 0 {
					return fmt.Errorf("--%s cannot be negative", flag)
				}
				if cmd.Flags().Changed(flag) {
					params.Set(strings.ReplaceAll(flag, "-", "_"), strconv.Itoa(value))
				}
			}
			if op.discovery {
				selectors, _ := cmd.Flags().GetStringArray("match")
				if cmd.Flags().Changed("match") && len(selectors) == 0 {
					return fmt.Errorf("--match cannot be empty")
				}
				if op.name == "series" && len(selectors) == 0 {
					return fmt.Errorf("series requires at least one --match selector")
				}
				for _, selector := range selectors {
					if strings.TrimSpace(selector) == "" {
						return fmt.Errorf("--match cannot be empty")
					}
					params.Add("match[]", selector)
				}
			}
			c, err := apiClient(cmd)
			if err != nil {
				return err
			}
			result, err := c.PromQL(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeAPIJSON(cmd, result)
		}
		parent.AddCommand(cmd)
	}
	return parent
}

func init() { rootCmd.AddCommand(newPromQLCommand()) }
