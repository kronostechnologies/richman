package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var Version = "latest"
var GitCommit = "hook"
var rootCmd = &cobra.Command{
	Use:          "richman",
	Short:        "Helmsman repository manager tool",
	Long:        "Helmsman repository manager tool",
	//SilenceUsage: true,
}

func init() {
	rootCmd.Version = fmt.Sprintf("%s-%s", Version, GitCommit)
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(chartCmd)
}

func Execute() error {
	return rootCmd.Execute()
}