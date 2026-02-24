package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "esql",
	Short: "Query Elasticsearch using ES|QL from the command line",
	Long: `esql is a CLI tool for running ES|QL queries against remote
Elasticsearch clusters. Use it for one-shot queries or launch
an interactive REPL for exploratory data analysis.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.esql.yaml)")
	rootCmd.PersistentFlags().String("host", "http://localhost:9200", "Elasticsearch host URL")
	rootCmd.PersistentFlags().String("user", "", "Elasticsearch username")
	rootCmd.PersistentFlags().String("password", "", "Elasticsearch password")
	rootCmd.PersistentFlags().String("api-key", "", "Elasticsearch API key (takes precedence over user/password)")
	rootCmd.PersistentFlags().Bool("insecure", false, "skip TLS certificate verification")
	rootCmd.PersistentFlags().String("profile", "", "named connection profile from config file")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
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
