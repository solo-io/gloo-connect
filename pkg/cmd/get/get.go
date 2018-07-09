package get

import (
	"github.com/solo-io/gloo-connect/pkg/cmd/util"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/spf13/cobra"
)

func Cmd(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get services available over http",
	}
	cmd.AddCommand(cmdGetServices(rc))
	return cmd
}

func cmdGetServices(rc *runner.RunConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "services",
		Short: "get services available over http",
		Run: func(c *cobra.Command, args []string) {
			util.PrintConsulServices(&rc.Options.ConsulOptions)
		},
	}
}
