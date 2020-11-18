package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"path"
)

var configFile string
var stateFile string

var Version = "latest"
var GitCommit = "hook"
var rootCmd = &cobra.Command{
	Use:   "richman",
	Short: "Helmsman repository manager tool",
	Long:  "Helmsman repository manager tool",
	PersistentPreRun: validatePersistentFlags,
	//SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (defaults to $HOME/.config/richman.yaml)")
	rootCmd.PersistentFlags().StringP("state-path", "p", "", "state path, defaults to cwd")
	rootCmd.PersistentFlags().StringP("state-file", "f", "", "state file (defaults to {current-context}.toml)")
	viper.BindPFlag("statePath", rootCmd.PersistentFlags().Lookup("state-path"))

	rootCmd.Version = fmt.Sprintf("%s-%s", Version, GitCommit)
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(chartCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {
	if configFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFile)
	} else {
		// Find home directory.
		if home, err := homedir.Dir() ; err == nil {
			viper.AddConfigPath(path.Join(home, ".config"))
		}
		viper.AddConfigPath("/etc/")
		viper.SetConfigName("richman")
	}

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func validatePersistentFlags(cmd *cobra.Command, args []string) {
	stateFile, err := cobra.PersistentFlags().GetString("state-file")
	fmt.Println(stateFile)
	fmt.Println(err)
}