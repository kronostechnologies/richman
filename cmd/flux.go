package cmd

import (
	"github.com/spf13/cobra"
)

var fluxCmd = &cobra.Command{
	Use:   "flux",
	Short: "image related command for flux",
	Long:  "Allow you to modifiy on the fly which version of a certain image you want reconcilied within your cluster",
}

func init() {
	appsCmd.AddCommand(fluxUpdateImage)
}
