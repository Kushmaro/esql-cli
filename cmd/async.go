package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/formatter"
	"github.com/spf13/cobra"
)

func apiClient(cmd *cobra.Command) (*client.Client, error) {
	if err := requireConnection(cmd); err != nil {
		return nil, err
	}
	cfg, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout < 0 {
		return nil, fmt.Errorf("--timeout cannot be negative")
	}
	cfg.Timeout = timeout
	return client.New(cfg), nil
}

func newAsyncCommand() *cobra.Command {
	parent := &cobra.Command{Use: "async", Short: "Submit, retrieve, stop, or delete asynchronous ES|QL queries"}
	submit := &cobra.Command{
		Use: "submit [query | -]", Short: "Submit an asynchronous query and return its ID and status", Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		Long: `Submit one query as an argument, from stdin with '-', or from --file.
The default output is the complete JSON response, including id and is_running.
Results are retained by default; use 'esql async delete <id>' to release them.
Native parameters, query settings and legacy --param substitution are supported.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := apiClient(cmd)
			if err != nil {
				return err
			}
			file, _ := cmd.Flags().GetString("file")
			queries, err := resolveQueries(args, file)
			if err != nil {
				return err
			}
			if len(queries) != 1 {
				return fmt.Errorf("async submit requires exactly one query")
			}
			params, _ := cmd.Flags().GetStringArray("param")
			query, err := config.ApplyParams(queries[0], params)
			if err != nil {
				return err
			}
			request, err := queryRequest(cmd, query)
			if err != nil {
				return err
			}
			opts, err := resultOptions(cmd, true)
			if err != nil {
				return err
			}
			wait, _ := cmd.Flags().GetString("wait-for-completion")
			keepAlive, _ := cmd.Flags().GetString("keep-alive")
			keep, _ := cmd.Flags().GetBool("keep-on-completion")
			resp, err := c.SubmitAsync(cmd.Context(), client.AsyncQueryRequest{QueryRequest: request, WaitForCompletionTimeout: wait, KeepAlive: keepAlive, KeepOnCompletion: &keep})
			if err != nil {
				return err
			}
			// Always emit the ID even when result formatting cannot finish yet.
			if resp.ID != "" && !opts.Raw {
				fmt.Fprintf(cmd.ErrOrStderr(), "Async query ID: %s\n", resp.ID)
			}
			return writeAsyncResult(cmd, resp, opts)
		},
	}
	addNativeQueryFlags(submit)
	addResultFlags(submit)
	submit.Flags().String("file", "", "read a single query from a file")
	submit.Flags().StringArray("param", nil, "text substitution key=value for {{key}} placeholders")
	submit.Flags().String("wait-for-completion", "1s", "initial time to wait for query results")
	submit.Flags().String("keep-alive", "5d", "how long Elasticsearch retains the query and results")
	submit.Flags().Bool("keep-on-completion", true, "retain results even if the query completes during the initial wait")
	submit.Flags().Duration("timeout", 30*time.Second, "HTTP request timeout")
	parent.AddCommand(submit)
	for _, operation := range []string{"get", "stop", "delete"} {
		operation := operation
		command := &cobra.Command{Use: operation + " <id>", Short: map[string]string{"get": "Get an async query's status and results", "stop": "Stop an async query and return available results", "delete": "Delete an async query and its stored results"}[operation], Args: cobra.ExactArgs(1), SilenceUsage: true}
		if operation != "delete" {
			command.Long = command.Short + `.
The default is the full JSON response, including running/partial status and metadata.
Specify --format to format completed result rows instead.`
		}
		command.RunE = func(cmd *cobra.Command, args []string) error {
			c, err := apiClient(cmd)
			if err != nil {
				return err
			}
			if operation == "delete" {
				result, err := c.DeleteAsync(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return writeAPIJSON(cmd, result)
			}
			opts, err := resultOptions(cmd, true)
			if err != nil {
				return err
			}
			var resp *client.QueryResponse
			if operation == "get" {
				wait, _ := cmd.Flags().GetString("wait-for-completion")
				keep, _ := cmd.Flags().GetString("keep-alive")
				resp, err = c.GetAsync(cmd.Context(), args[0], client.AsyncGetOptions{WaitForCompletionTimeout: wait, KeepAlive: keep})
			} else {
				resp, err = c.StopAsync(cmd.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return writeAsyncResult(cmd, resp, opts)
		}
		command.Flags().Duration("timeout", 30*time.Second, "HTTP request timeout")
		if operation == "delete" {
			command.Flags().StringP("output", "o", "", "write response JSON to a file")
		} else {
			addResultFlags(command)
		}
		if operation == "get" {
			command.Flags().String("wait-for-completion", "0s", "time to wait for completion; 0s checks immediately")
			command.Flags().String("keep-alive", "", "extend the result retention period")
		}
		parent.AddCommand(command)
	}
	return parent
}

func writeAsyncResult(cmd *cobra.Command, resp *client.QueryResponse, opts formatter.Options) error {
	if resp.IsRunning && !opts.Raw {
		return fmt.Errorf("query is still running; use 'esql async get <id>' for status or retrieve result rows after completion")
	}
	output, _ := cmd.Flags().GetString("output")
	if err := validateParquetOutput(opts.Format, output, 1); err != nil {
		return err
	}
	return writeResults(output, opts.Format, func(w io.Writer) error { return formatter.Format(w, resp, opts) })
}

func writeAPIJSON(cmd *cobra.Command, result json.RawMessage) error {
	output, _ := cmd.Flags().GetString("output")
	return writeResults(output, "json", func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	})
}

func newRunningCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "running [id]", Short: "List running ES|QL queries or inspect one by its monitoring ID", Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		Long: `Return running-query information as JSON. With no ID, list running queries.
With an ID from that list, inspect its coordinating and data nodes.
Use a query ID from the list; completed async results are retrieved with 'async get'.
Requires the Elasticsearch monitor_esql cluster privilege.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := apiClient(cmd)
			if err != nil {
				return err
			}
			var result json.RawMessage
			if len(args) == 0 {
				result, err = c.ListRunningQueries(cmd.Context())
			} else {
				result, err = c.GetRunningQuery(cmd.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return writeAPIJSON(cmd, result)
		},
	}
	cmd.Flags().Duration("timeout", 30*time.Second, "HTTP request timeout")
	cmd.Flags().StringP("output", "o", "", "write response JSON to a file")
	return cmd
}

func init() { rootCmd.AddCommand(newAsyncCommand(), newRunningCommand()) }
