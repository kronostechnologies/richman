package cmd

import (
	"github.com/spf13/cobra"
)

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "app-related commands",
	Long: "App-related commands",
}

func init() {
	appsCmd.AddCommand(appsListCmd)
}

