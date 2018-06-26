package runner

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/signal"
	"time"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/consul/api"
	"github.com/solo-io/gloo-consul-bridge/pkg/consul"
	"github.com/solo-io/gloo-consul-bridge/pkg/envoy"
	"github.com/solo-io/gloo-consul-bridge/pkg/gloo"
	"github.com/solo-io/gloo/pkg/storage"
)

func cancelOnTerm(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		signal.Reset(os.Interrupt)
		cancel()
	}()
	return ctx, cancel
}

func Run(runconfig RunConfig, store storage.Interface) error {
	if runconfig.ConfigDir == "" {
		var err error
		runconfig.ConfigDir, err = ioutil.TempDir("", "")
		if err != nil {
			return err
		}
		defer os.RemoveAll(runconfig.ConfigDir)
	}

	// get what we need from consul
	cfg, err := consul.NewConsulConnectConfigFromEnv()
	if err != nil {
		return err
	}
	// TODO(ilackarms): do not hard-code
	rolename, configWriter := gloo.NewConfigWriter(store, cfg, gloo.ConsulInfo{
		ConsulHostname: "localhost",
		ConsulPort:     8500,
		AuthorizePath:  "/v1/agent/connect/authorize",
		ConfigDir: runconfig.ConfigDir,
	})

	ctx := context.Background()
	ctx, cancelTerm := cancelOnTerm(ctx)
	defer cancelTerm()

	cf, err := consul.NewCertificateFetcher(ctx, configWriter, cfg)
	if err != nil {
		return err
	}

	// we need one root cert and client cert to begin:
	rootcert := <-cf.RootCerts()
	leaftcert := <-cf.Certs()

	id := &envoycore.Node{
		Id:      rolename + "~" + getNodeName(),
		Cluster: cfg.ProxyId(),
	}

	e := envoy.NewEnvoy(runconfig.EnvoyPath, runconfig.GlooAddress, runconfig.GlooPort, runconfig.ConfigDir, id)
	envoyCfg := envoy.Config{
		LeafCert: leaftcert,
		RootCas:  rootcert,
	}
	err = e.WriteConfig(envoyCfg)
	if err != nil {
		return errors.New("can't write config")
	}
	err = e.Reload()
	if err != nil {
		return errors.New("can't start envoy config")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case rootcert = <-cf.RootCerts():
				envoyCfg.RootCas = rootcert
			case leaftcert = <-cf.Certs():
				envoyCfg.LeafCert = leaftcert
			}
			err = e.WriteConfig(envoyCfg)
			if err != nil {
				// TODO: log this
				// return errors.New("can't write config")
				return
			}
			EventuallyReload(e)
		}
	}()

	if err := e.Run(ctx); err != nil {
		return err
	}
	return ctx.Err()
}

func EventuallyReload(e envoy.Envoy) {
	for {
		err := e.Reload()
		if err == nil {
			return
		}
		time.Sleep(10 * time.Second)
	}
}

func getNodeName() string {
	consulConfig := api.DefaultConfig()
	client, err := api.NewClient(consulConfig)
	if err == nil {
		name, err := client.Agent().NodeName()
		if err == nil {
			return name
		}
	}
	name, err := os.Hostname()
	if err == nil {
		return name
	}

	return "generic-node"
}
