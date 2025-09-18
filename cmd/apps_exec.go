package cmd

import (
	"errors"
	"regexp"

	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
)

var appsExecCmd = &cobra.Command{
	Use:   "exec <command> -a APPLICATION",
	Short: "execute command in app ops env and output logs",
	Long:  "execute command in app ops env and output logs instead of attaching to pod",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("exactly one command argument is required")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		app_filters, _ := cmd.Flags().GetString("app")
		if app_filters == "" {
			return errors.New("please provide an application to run with the -a flag")
		}
		configArgs, _ := cmd.Flags().GetStringArray("config")

		splitRegex := regexp.MustCompile(`^([^=]+)=(.*)$`)

		configs := make(map[string]string)

		for _, configArg := range configArgs {
			split := splitRegex.FindStringSubmatch(configArg)
			if len(split) != 3 {
				return errors.New("config must be in format key=value")
			}
			key := split[1]
			value := split[2]
			configs[key] = value
		}

		c := action.AppsExec{
			Application: app_filters,
			Command:     args[0],
			Config:      configs,
		}

		return c.Run()
	},
}

func init() {
	appsExecCmd.Flags().StringP("app", "a", "", "select app by name")
	appsExecCmd.Flags().StringArrayP("config", "c", []string{}, "set config key=value")
}
