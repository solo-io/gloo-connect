package bridge

import (
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap/configstorage"
	"github.com/spf13/cobra"
)

func Cmd(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "runs gloo-connect to bridge Envoy to Consul's connect api",
		RunE: func(_ *cobra.Command, args []string) error {
			return run(rc)
		},
	}

	cmd.PersistentFlags().StringVar(&rc.GlooAddress, "gloo-address", "127.0.0.1", "bind address where gloo should serve xds config to envoy")
	cmd.PersistentFlags().UintVar(&rc.GlooPort, "gloo-port", 8081, "port where gloo should serve xds config to envoy")
	cmd.PersistentFlags().BoolVar(&rc.UseUDS, "gloo-uds", false, "use unix domain socket for gloo and envoy")
	cmd.PersistentFlags().StringVar(&rc.ConfigDir, "conf-dir", "", "config dir to hold envoy config file")
	cmd.PersistentFlags().StringVar(&rc.EnvoyPath, "envoy-path", "", "path to envoy binary")
	return cmd
}

func run(rc *runner.RunConfig) error {
	store, err := configstorage.Bootstrap(rc.Options)
	if err != nil {
		return err
	}
	if err := runner.Run(*rc, store); err != nil {
		return err
	}
	return nil
}
