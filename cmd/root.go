package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "richman",
	Short: "Helmsman repository manager tool",
	Long:  "Helmsman repository manager tool",
}

func init() {
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(chartCmd)
	rootCmd.AddCommand(imageCmd)
}

func SetVersion(version string) {
	rootCmd.Version = version
}

func Execute() error {
	return rootCmd.Execute()
}
