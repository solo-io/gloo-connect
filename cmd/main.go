package main

import (
	"fmt"
	"os"

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

var (
	opts = &bootstrap.Options{}
	rc   runner.RunConfig
)

var rootCmd = &cobra.Command{
	Use:   "gloo-consul-bridge",
	Short: "runs the gloo-consul bridge to connect gloo to consul's connect api",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func run() error {
	store, err := configstorage.Bootstrap(*opts)
	if err != nil {
		return err
	}
	if err := runner.Run(rc, store); err != nil {
		return err
	}
	return nil
}

func init() {
	// choose storage options (type, etc) for configs, secrets, and artifacts
	flags.AddConfigStorageOptionFlags(rootCmd, opts)
	flags.AddSecretStorageOptionFlags(rootCmd, opts)
	flags.AddFileStorageOptionFlags(rootCmd, opts)

	// storage backends
	flags.AddFileFlags(rootCmd, opts)
	flags.AddKubernetesFlags(rootCmd, opts)
	flags.AddConsulFlags(rootCmd, opts)
	flags.AddVaultFlags(rootCmd, opts)

	rootCmd.PersistentFlags().StringVar(&rc.GlooAddress, "gloo-address", "", "address for gloo ADS server")
	rootCmd.PersistentFlags().UintVar(&rc.GlooPort, "gloo-port", 0, "port for gloo ADS server")
	rootCmd.PersistentFlags().StringVar(&rc.ConfigDir, "conf-dir", "", "config dir to hold envoy config file")
	rootCmd.PersistentFlags().StringVar(&rc.EnvoyPath, "envoy-path", "", "path to envoy binary")
}
