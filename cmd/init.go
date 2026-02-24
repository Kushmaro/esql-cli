package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize connection settings",
	Long:  `Interactively configure Elasticsearch connection settings and save them to ~/.esql.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("init command not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
