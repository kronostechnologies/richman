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

func init() {
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(chartCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}