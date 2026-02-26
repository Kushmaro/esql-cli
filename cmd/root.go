package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/Kushmaro/esql-cli/internal/repl"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const (
	ExitSuccess    = 0
	ExitQuery      = 1
	ExitConnection = 2
	ExitConfig     = 3
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "esql",
	Short: "Query Elasticsearch using ES|QL from the command line",
	Long: `esql is a CLI tool for running ES|QL queries against remote
Elasticsearch clusters. Run with no subcommand to launch an
interactive shell with tab completion and history.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
			if err := requireConnection(cmd); err != nil {
				return err
			}
			cfg, err := config.ClientConfig()
			if err != nil {
				return err
			}
			r := repl.New(cfg)
			return r.Run()
		}
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if hint := client.ErrorHint(err); hint != "" {
			fmt.Fprintf(os.Stderr, "\n  Hint: %s\n", hint)
		}
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	var connErr *client.ConnectionError
	if errors.As(err, &connErr) {
		return ExitConnection
	}
	var esErr *client.ESError
	if errors.As(err, &esErr) {
		return ExitQuery
	}
	return ExitQuery
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.esql.yaml)")
	rootCmd.PersistentFlags().String("host", "http://localhost:9200", "Elasticsearch host URL")
	rootCmd.PersistentFlags().String("user", "", "Elasticsearch username")
	rootCmd.PersistentFlags().String("password", "", "Elasticsearch password")
	rootCmd.PersistentFlags().String("api-key", "", "Elasticsearch API key (takes precedence over user/password)")
	rootCmd.PersistentFlags().Bool("insecure", false, "skip TLS certificate verification")
	rootCmd.PersistentFlags().String("cluster", "", "named cluster connection from config file")
	rootCmd.PersistentFlags().Bool("verbose", false, "show HTTP request/response details for debugging")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	viper.BindPFlag("cluster", rootCmd.PersistentFlags().Lookup("cluster"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// requireConnection returns an error if no Elasticsearch connection has been
// configured and no explicit connection flags were passed. Commands that need
// ES connectivity call this before attempting to connect.
func requireConnection(cmd *cobra.Command) error {
	pf := cmd.Root().PersistentFlags()
	for _, name := range []string{"host", "user", "password", "api-key", "cluster"} {
		if pf.Changed(name) {
			return nil
		}
	}
	if config.HasConnectionConfig() {
		return nil
	}
	return fmt.Errorf("no Elasticsearch connection configured\n\nRun 'esql init' to set up a connection, or provide --host and credentials via flags.")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".esql")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("ESQL")
	viper.AutomaticEnv()

	viper.ReadInConfig()
}
