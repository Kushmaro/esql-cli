package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize connection settings",
	Long: `Interactively configure Elasticsearch connection settings.
Prompts for host, authentication method, and credentials,
tests the connection, and saves the cluster to ~/.esql.yaml.`,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	clusterName := prompt(reader, "Cluster name", "default")

	host := prompt(reader, "Elasticsearch host", "http://localhost:9200")

	fmt.Println("\nAuth method:")
	fmt.Println("  1) Basic (user/password)")
	fmt.Println("  2) API Key")
	fmt.Println("  3) None")
	authChoice := prompt(reader, "Choose [1/2/3]", "1")

	var user, password, apiKey string
	switch authChoice {
	case "1":
		user = prompt(reader, "Username", "elastic")
		password = promptSecret(reader, "Password")
	case "2":
		apiKey = prompt(reader, "API Key (base64-encoded)", "")
		if apiKey == "" {
			return fmt.Errorf("API key cannot be empty")
		}
	case "3":
		// no auth
	default:
		return fmt.Errorf("invalid choice: %s", authChoice)
	}

	insecureStr := prompt(reader, "Skip TLS verification? [y/N]", "N")
	insecure := strings.ToLower(insecureStr) == "y" || strings.ToLower(insecureStr) == "yes"

	cfg := client.Config{
		Host:     host,
		User:     user,
		Password: password,
		APIKey:   apiKey,
		Insecure: insecure,
	}

	fmt.Fprintf(os.Stderr, "\nTesting connection to %s... ", host)
	c := client.New(cfg)
	info, err := c.Ping()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAILED")
		return fmt.Errorf("connection test failed: %w", err)
	}
	if info.Version.Number != "" {
		fmt.Fprintf(os.Stderr, "OK (cluster: %s, version: %s)\n", info.Cluster, info.Version.Number)
	} else {
		fmt.Fprintf(os.Stderr, "OK (cluster info unavailable — read-only credentials)\n")
	}

	fileCfg, err := config.LoadFileConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fileCfg.Clusters[clusterName] = config.Cluster{
		Host:     host,
		User:     user,
		Password: password,
		APIKey:   apiKey,
		Insecure: insecure,
	}

	if len(fileCfg.Clusters) == 1 || fileCfg.DefaultCluster == "" {
		fileCfg.DefaultCluster = clusterName
	} else {
		setDefault := prompt(reader, fmt.Sprintf("Set '%s' as default cluster? [y/N]", clusterName), "N")
		if strings.ToLower(setDefault) == "y" || strings.ToLower(setDefault) == "yes" {
			fileCfg.DefaultCluster = clusterName
		}
	}

	if err := config.SaveFileConfig(fileCfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	path, _ := config.ConfigPath()
	fmt.Fprintf(os.Stderr, "\nCluster '%s' saved to %s\n", clusterName, path)

	if fileCfg.DefaultCluster == clusterName {
		fmt.Fprintf(os.Stderr, "Set as default cluster.\n")
	}

	fmt.Fprintf(os.Stderr, "\nYou can now run:\n")
	fmt.Fprintf(os.Stderr, "  esql query 'FROM my-index | LIMIT 10'\n")
	if clusterName != "default" {
		fmt.Fprintf(os.Stderr, "  esql query --cluster %s 'FROM my-index | LIMIT 10'\n", clusterName)
	}

	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func promptSecret(reader *bufio.Reader, label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
