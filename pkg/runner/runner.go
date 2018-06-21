package runner

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"time"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/consul/api"
	"github.com/solo-io/consul-gloo-bridge/pkg/consul"
	"github.com/solo-io/consul-gloo-bridge/pkg/envoy"
	"github.com/solo-io/consul-gloo-bridge/pkg/gloo"
)

func Run(runconfig RunConfig) error {

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
	rolename, configWriter := gloo.NewConfigWriter(cfg)

	ctx := context.Background()
	cf, err := consul.NewCertificateFetcher(ctx, configWriter, cfg)
	if err != nil {
		return err
	}

	// we need one root cert and client cert to begin:
	rootcert := <-cf.RootCerts()
	leaftcert := <-cf.Certs()

	id := &envoycore.Node{
		Id:      getNodeName(),
		Cluster: cfg.ProxyId() + "~" + rolename,
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
	go func() {
		e.Run()
		cancel()
	}()
	defer cancel()
	defer e.Exit()

	for {
		select {
		case <-ctx.Done():
			return nil
		case rootcert = <-cf.RootCerts():
			envoyCfg.RootCas = rootcert
		case leaftcert = <-cf.Certs():
			envoyCfg.LeafCert = leaftcert
		}
		err = e.WriteConfig(envoyCfg)
		if err != nil {
			return errors.New("can't write config")
		}
		EventuallyReload(e)
	}
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
