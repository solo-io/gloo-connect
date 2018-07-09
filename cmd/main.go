package main

import (
	"fmt"
	"os"

	"github.com/solo-io/gloo-connect/pkg/cmd/bridge"
	"github.com/solo-io/gloo-connect/pkg/cmd/get"
	"github.com/solo-io/gloo-connect/pkg/cmd/http"
	"github.com/solo-io/gloo-connect/pkg/cmd/set"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap"
	"github.com/solo-io/gloo/pkg/bootstrap/flags"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rc = runner.RunConfig{}

var rootCmd = &cobra.Command{
	Use:   "gloo-connect",
	Short: "root command for running and managing gloo-connect",
}

func initRunnerConfig(c *runner.RunConfig) {
	// always use consul for storage and service discovery
	c.Options.ConfigStorageOptions.Type = bootstrap.WatcherTypeConsul
	c.Options.FileStorageOptions.Type = bootstrap.WatcherTypeConsul
}

func init() {
	// for storage and service discovery
	flags.AddConsulFlags(rootCmd, &rc.Options)

	initRunnerConfig(&rc)

	rootCmd.AddCommand(bridge.Cmd(&rc), http.Cmd(&rc), get.Cmd(&rc), set.Cmd(&rc))
}
