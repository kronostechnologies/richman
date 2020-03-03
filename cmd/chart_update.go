package cmd

import (
	"errors"
	"fmt"
	"github.com/konostechnologies/richman/action"
	"github.com/spf13/cobra"
	"os"
)

var chartUpdateCmd = &cobra.Command{
	Use:   "update FILENAME",
	Short: "Chart Update ops",
	Long:  "Long Chart Ops",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("filename required")
		}
		if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("invalid filename specified: %s", args[0])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		app_filters, _ := cmd.Flags().GetStringArray("app")
		chart_filters, _ := cmd.Flags().GetStringArray("chart")
		apply, _ := cmd.Flags().GetBool("apply")
		skip_repo_update, _ := cmd.Flags().GetBool("skip-repo-update")

		c := action.ChartUpdate{
			Filename:     args[0],
			AppFilters:   app_filters,
			ChartFilters: chart_filters,
			Apply:        apply,
			RepoUpdate:   !skip_repo_update,
		}

		return c.Run()
	},

}

func init(){
	chartUpdateCmd.Flags().Bool( "apply",  false, "apply update")
	chartUpdateCmd.Flags().Bool( "skip-repo-update",  false, "skip helm repository update")
	chartUpdateCmd.Flags().StringArrayP( "chart",  "c", []string{}, "select repo/chart by name")
	chartUpdateCmd.Flags().StringArrayP( "app",  "a", []string{}, "select app by name")
}