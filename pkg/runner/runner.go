package runner

import (
	"context"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/solo-io/consul-gloo-bridge/pkg/consul"
	"github.com/solo-io/consul-gloo-bridge/pkg/envoy"
	"github.com/solo-io/consul-gloo-bridge/pkg/gloo"
)

func Run() error {
	var configWriter consul.ConfigWriter = &gloo.GlooConfigWriter{}
	// get what we need from consul
	cfg, err := consul.NewConsulConnectConfigFromEnv()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cf, err := consul.NewCertificateFetcher(ctx, configWriter, cfg)
	if err != nil {
		return err
	}

	// we need one root cert and client cert to begin:
	rootcert := <-cf.RootCerts()
	leaftcert := <-cf.Certs()

	glooAddress := "fake"
	glooPort := uint(34)
	configDir := "fake"
	id := &envoycore.Node{
		Id: "fake",
	}

	e := envoy.NewEnvoy(glooAddress, glooPort, configDir, id)
	envoyCfg := envoy.Config{
		LeafCert: leaftcert,
		RootCas:  rootcert,
	}
	e.WriteConfig(envoyCfg)
	e.Reload()

	for {
		select {
		case <-ctx.Done():
			return nil
		case rootcert = <-cf.RootCerts():
			envoyCfg.RootCas = rootcert
		case leaftcert = <-cf.Certs():
			envoyCfg.LeafCert = leaftcert
		}
		e.WriteConfig(envoyCfg)

	}
}
