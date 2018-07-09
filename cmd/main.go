package main

import (
	"fmt"
	"os"

	"github.com/solo-io/gloo-connect/pkg/cmd"
	"github.com/solo-io/gloo-connect/pkg/cmd/get"
	"github.com/solo-io/gloo-connect/pkg/cmd/set"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap"
	"github.com/solo-io/gloo/pkg/bootstrap/configstorage"
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

func run() error {
	store, err := configstorage.Bootstrap(rc.Options)
	if err != nil {
		return err
	}
	if err := runner.Run(rc, store); err != nil {
		return err
	}
	return nil
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

	bridgeCmd.PersistentFlags().StringVar(&rc.GlooAddress, "gloo-address", "127.0.0.1", "bind address where gloo should serve xds config to envoy")
	bridgeCmd.PersistentFlags().UintVar(&rc.GlooPort, "gloo-port", 8081, "port where gloo should serve xds config to envoy")
	bridgeCmd.PersistentFlags().BoolVar(&rc.UseUDS, "gloo-uds", false, "use unix domain socket for gloo and envoy")
	bridgeCmd.PersistentFlags().StringVar(&rc.ConfigDir, "conf-dir", "", "config dir to hold envoy config file")
	bridgeCmd.PersistentFlags().StringVar(&rc.EnvoyPath, "envoy-path", "", "path to envoy binary")

	rootCmd.AddCommand(bridgeCmd, httpCmd, get.Cmd(&rc), set.Cmd(&rc))
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "runs gloo-connect to bridge Envoy to Consul's connect api",
	RunE: func(_ *cobra.Command, args []string) error {
		return run()
	},
}

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "manage HTTP features for in-mesh services",
	Long:  "",
	RunE: func(_ *cobra.Command, args []string) error {
		store, err := configstorage.Bootstrap(rc.Options)
		if err != nil {
			return err
		}

		gc := cmd.GlooClient{Store: store}

		return gc.Demo()
	},
}
