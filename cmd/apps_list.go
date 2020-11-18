package cmd

import (
	"fmt"
	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var appsListCmd = &cobra.Command{
	Use:   "list FILENAME",
	Short: "list app versions",
	Long:  "List app versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		app_filters, _ := cmd.Flags().GetStringArray("app")

		c := action.AppsList{
			Filename:     stateFile,
			AppFilters:   app_filters,
		}

		fmt.Println(viper.GetString("statePath"))
		fmt.Println("stuff")

		return c.Run()
	},

}

func init(){
	appsListCmd.Flags().StringArrayP( "app",  "a", []string{}, "select app by name")
}