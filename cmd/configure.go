package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure connection settings",
	Long:  `Interactively configure Elasticsearch connection settings and save them to ~/.esql.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("configure command not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)
}
