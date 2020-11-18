package cmd

import (
	"errors"
	"fmt"
	"github.com/kronostechnologies/richman/action"
	"github.com/spf13/cobra"
	"os"
	"regexp"
)

var appsRunCmd = &cobra.Command{
	Use:   "run [FILENAME]",
	Short: "run app ops env",
	Long:  "run app ops env",
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
		appFilter, _ := cmd.Flags().GetString("app")
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
			Filename:    args[0],
			Application: appFilter,
			Config:      configs,
		}

		return c.Run()
	},

}

func init(){
	appsRunCmd.Flags().StringP( "app",  "a", "", "select app by name")
	appsRunCmd.Flags().StringArrayP( "config",  "c", []string{}, "set config key=value")
}