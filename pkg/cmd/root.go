package cmd

import (
	"github.com/solo-io/gloo-connect/pkg/cmd/bridge"
	"github.com/solo-io/gloo-connect/pkg/cmd/get"
	"github.com/solo-io/gloo-connect/pkg/cmd/http"
	"github.com/solo-io/gloo-connect/pkg/cmd/set"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap"
	"github.com/solo-io/gloo/pkg/bootstrap/flags"

	"github.com/spf13/cobra"
)

func Cmd(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gloo-connect",
		Short: "root command for running and managing gloo-connect",
	}
	// for storage and service discovery
	flags.AddConsulFlags(cmd, &rc.Options)

	initRunnerConfig(rc)
	cmd.AddCommand(bridge.Cmd(rc), http.Cmd(rc), get.Cmd(rc), set.Cmd(rc), completionCmd())
	return cmd
}

func initRunnerConfig(c *runner.RunConfig) {
	// always use consul for storage and service discovery
	c.Options.ConfigStorageOptions.Type = bootstrap.WatcherTypeConsul
	c.Options.FileStorageOptions.Type = bootstrap.WatcherTypeConsul
}
