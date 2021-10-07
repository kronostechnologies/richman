package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
)

var fluxUpdateImageCmd = &cobra.Command{
	Use:   "update cluster image",
	Short: "Update image version of app/HelmChart",
	Long:  "Update the version of the specified apps within your cluster, picked up by Flux and immediatly update it",
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
