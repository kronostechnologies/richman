package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var Version = ""
var GitCommit = ""
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version information",
	Long: "Show version information i.e. 'version' is the git tag or git commit if not build on a tag and 'gitcommit' is the git commit sha",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("version: %s\n", Version)
		fmt.Printf("gitcommit: %s\n", GitCommit)
  	},
}

