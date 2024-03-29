package cmd

import (
	"errors"
	"regexp"

	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
)

var appsRunCmd = &cobra.Command{
	Use:   "run -a APPLICATION",
	Short: "run app ops env",
	Long:  "run app ops env",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			//TODO : Default to current context, allow to append context
			return errors.New("too many arguments")
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
			key := split[1]
			value := split[2]
			configs[key] = value
		}
		c := action.AppsRun{
			Application: app_filters,
			Config:      configs,
		}

		return c.Run()
	},
}

func init() {
	appsRunCmd.Flags().StringP("app", "a", "", "select app by name")
	appsRunCmd.Flags().StringArrayP("config", "c", []string{}, "set config key=value")
}
