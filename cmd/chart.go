package cmd

import (
	"github.com/spf13/cobra"
)

var chartCmd = &cobra.Command{
	Use:   "chart",
	Short: "chart-related commands",
	Long: "Chart-related commands",
}

func init() {
	chartCmd.AddCommand(chartUpdateCmd)
}

