package cmd

import (
	"errors"

	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
)

var appsListCmd = &cobra.Command{
	Use:   "list apps and their versions",
	Short: "list app versions",
	Long:  "List app versions",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			//TODO : Default to current context, allow to append context
			return errors.New("too many arguments")
		}
		return nil
		//return fmt.Errorf("a generic error here: %s", args[0])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		app_filters, _ := cmd.Flags().GetStringArray("app")

		filters := action.AppFilters{
			Filters: app_filters,
		}

		return action.Run(filters)
	},
}

func init() {
	appsListCmd.Flags().StringArrayP("app", "a", []string{}, "select app by name")
}
