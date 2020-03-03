package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "richman",
	Short:        "Helmsman repository manager tool",
	Long:        "Helmsman repository manager tool",
	//SilenceUsage: true,
}


var Apply bool

func init() {
	rootCmd.AddCommand(chartCmd)
}

func Execute() error {
	return rootCmd.Execute()
}