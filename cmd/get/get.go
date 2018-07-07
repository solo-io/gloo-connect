package get

import (
	"github.com/solo-io/gloo-connect/cmd/util"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get services available over http",
	}
	cmd.AddCommand(cmdGetServices)
	return cmd
}

var cmdGetServices = &cobra.Command{
	Use:   "services",
	Short: "get services available over http",
	Run: func(c *cobra.Command, args []string) {
		util.PrintConsulServices()
	},
}
