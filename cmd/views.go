package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/spf13/cobra"
)

func addAPIPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Duration("timeout", 30*time.Second, "HTTP request timeout")
	cmd.PersistentFlags().StringP("output", "o", "", "write the full JSON response to a file")
}

// readSingleQuery deliberately preserves semicolons, quoted strings and SET
// directives. View definitions contain one complete query, not a query batch.
func readSingleQuery(cmd *cobra.Command, args []string) (string, error) {
	file, _ := cmd.Flags().GetString("file")
	if len(args) > 1 {
		return "", fmt.Errorf("provide one query argument")
	}
	if file != "" && len(args) > 0 {
		return "", fmt.Errorf("provide either a query argument or --file")
	}
	var data []byte
	var err error
	switch {
	case file != "":
		data, err = os.ReadFile(file)
	case len(args) == 1 && args[0] == "-":
		data, err = io.ReadAll(cmd.InOrStdin())
	case len(args) == 1:
		data = []byte(args[0])
	default:
		return "", fmt.Errorf("provide a query argument, '-' for stdin, or --file")
	}
	if err != nil {
		return "", fmt.Errorf("reading query: %w", err)
	}
	query := strings.TrimSpace(string(data))
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}
	return query, nil
}

func newViewsCommand() *cobra.Command {
	parent := &cobra.Command{Use: "views", Aliases: []string{"view"}, Short: "Manage server-side ES|QL stored views", Long: `Manage stored views on the configured Elasticsearch cluster.
Views are shared server objects. Local saved queries remain under 'save' and 'queries'.
Use esql query 'FROM <view-name>' to execute a view.
Required privileges: read_view_metadata for list/get, create_view for put,
 and delete_view for delete.`}
	addAPIPersistentFlags(parent)
	list := &cobra.Command{Use: "list", Short: "List stored views", Args: cobra.NoArgs, SilenceUsage: true}
	get := &cobra.Command{Use: "get <name> [name...]", Short: "Get stored view definitions", Args: cobra.MinimumNArgs(1), SilenceUsage: true}
	put := &cobra.Command{Use: "put <name> [query | -]", Short: "Create or replace a stored view", Args: cobra.RangeArgs(1, 2), SilenceUsage: true,
		Long: `Create or replace a server-side view using a query argument, stdin ('-'),
or --file. The query is sent verbatim. --description can set or clear its description on servers that support that field.
Examples:
  esql views put my-view 'ROW n = 42' --description 'Example view'
  esql views put my-view --file definition.esql
  esql query 'FROM my-view'`,
	}
	put.Flags().String("file", "", "read the complete view query from a file")
	put.Flags().String("description", "", "view description if supported by the server (empty clears it)")
	deleteCmd := &cobra.Command{Use: "delete <name> [name...]", Short: "Delete stored views", Args: cobra.MinimumNArgs(1), SilenceUsage: true}
	for _, cmd := range []*cobra.Command{list, get, put, deleteCmd} {
		operation := cmd.Name()
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			var definition client.ViewRequest
			if operation == "put" {
				query, err := readSingleQuery(cmd, args[1:])
				if err != nil {
					return err
				}
				definition.Query = query
				if cmd.Flags().Changed("description") {
					description, _ := cmd.Flags().GetString("description")
					definition.Description = &description
				}
			}
			c, err := apiClient(cmd)
			if err != nil {
				return err
			}
			var result json.RawMessage
			switch operation {
			case "list":
				result, err = c.GetViews(cmd.Context(), "")
			case "get":
				result, err = c.GetViews(cmd.Context(), strings.Join(args, ","))
			case "put":
				result, err = c.PutView(cmd.Context(), args[0], definition)
			case "delete":
				result, err = c.DeleteViews(cmd.Context(), strings.Join(args, ","))
			}
			if err != nil {
				return err
			}
			return writeAPIJSON(cmd, result)
		}
		parent.AddCommand(cmd)
	}
	return parent
}

func init() { rootCmd.AddCommand(newViewsCommand()) }
