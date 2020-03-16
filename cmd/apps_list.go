package cmd

import (
	"errors"
	"fmt"
	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
	"os"
)

var appsListCmd = &cobra.Command{
	Use:   "list FILENAME",
	Short: "list app versions",
	Long:  "List app versions",
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

		c := action.AppsList{
			Filename:     args[0],
			AppFilters:   app_filters,
		}

		return c.Run()
	},

}

func init(){
	appsListCmd.Flags().StringArrayP( "app",  "a", []string{}, "select app by name")
}