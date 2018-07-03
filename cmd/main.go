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
	rc   runner.RunConfig
)

var rootCmd = &cobra.Command{
	Use:   "gloo-connect",
	Short: "runs gloo-connect to bridge Envoy to Consul's connect api",
	RunE: func(cmd *cobra.Command, args []string) error {
		// defaults
		rc.GlooAddress = "127.0.0.1"
		rc.GlooPort = 8081
		rc.Options.ConfigStorageOptions.Type = bootstrap.WatcherTypeConsul
		rc.Options.FileStorageOptions.Type = bootstrap.WatcherTypeConsul

		// secrets isn't used anyway - only do in-memory for now
		// TODO(ilackarms): support a flag for in-memory storage
		rc.Options.SecretStorageOptions.Type = bootstrap.WatcherTypeFile
		return run()
	},
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

func init() {
	// for storage and service discovery
	flags.AddConsulFlags(rootCmd, &rc.Options)
	// not used, kombina to prevent secret storage from complaining
	// TODO(ilackarms): support a flag for in-memory storage
	flags.AddFileFlags(rootCmd, &rc.Options)

	rootCmd.PersistentFlags().StringVar(&rc.ConfigDir, "conf-dir", "", "config dir to hold envoy config file")
	rootCmd.PersistentFlags().StringVar(&rc.EnvoyPath, "envoy-path", "", "path to envoy binary")
}
