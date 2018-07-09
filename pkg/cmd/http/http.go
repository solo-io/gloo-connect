package http

import (
	rootcmd "github.com/solo-io/gloo-connect/pkg/cmd"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap/configstorage"
	"github.com/spf13/cobra"
)

func Cmd(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http",
		Short: "manage HTTP features for in-mesh services",
		Long:  "",
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := configstorage.Bootstrap(rc.Options)
			if err != nil {
				return err
			}

			gc := rootcmd.GlooClient{Store: store}

			return gc.Demo()
		},
	}
	return cmd
}
