package set

// maybe call this parameter "activate"?
import (
	"errors"
	glooconnectcmd "github.com/solo-io/gloo-connect/pkg/cmd"
	"github.com/solo-io/gloo-connect/pkg/runner"
	"github.com/solo-io/gloo/pkg/bootstrap/configstorage"
	"github.com/spf13/cobra"
)

type serviceFlagsType struct {
	retries uint32
	http    bool
}

var serviceFlags = serviceFlagsType{}

func Cmd(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set service",
	}
	cmd.AddCommand(cmdSetServices(rc))
	return cmd
}

func cmdSetServices(rc *runner.RunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "set service",
		RunE: func(c *cobra.Command, args []string) error {
			err := validateServiceParams(serviceFlags, args)
			if err != nil {
				return err
			}
			store, err := configstorage.Bootstrap(rc.Options)
			if err != nil {
				return err
			}

			gc := glooconnectcmd.GlooClient{Store: store}

			return gc.ConfigureService(args[0], serviceFlags.retries)
		},
	}
	cmd.PersistentFlags().Uint32VarP(&serviceFlags.retries, "retries", "", 0, "number of retries")
	cmd.PersistentFlags().BoolVarP(&serviceFlags.http, "http", "", false, "whether http mode should be used, default false")
	return cmd
}

func validateSetServiceArgs(args []string) error {
	if len(args) != 1 {
		return errors.New("must pass a single argument")
	}
	return nil
}

func validateServiceParams(flags serviceFlagsType, args []string) error {
	if len(args) != 1 {
		return errors.New("must pass a single argument")
	}
	// if !flags.http {
	// 	// TODO(mitchdraft) Do we want this constraint? Or flag?
	// 	return errors.New("must explicitly specify the --http flag when using `set service`")
	// }
	return nil
}
