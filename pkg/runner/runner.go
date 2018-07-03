package runner

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/consul/api"
	"github.com/solo-io/gloo-connect/pkg/consul"
	"github.com/solo-io/gloo-connect/pkg/envoy"
	"github.com/solo-io/gloo-connect/pkg/gloo"
	"github.com/solo-io/gloo/pkg/log"
	"github.com/solo-io/gloo/pkg/storage"
	localstorage "github.com/solo-io/gloo-connect/pkg/storage"
	"github.com/solo-io/gloo/pkg/bootstrap/artifactstorage"
	pkgerrs "github.com/pkg/errors"
	"github.com/solo-io/gloo/pkg/control-plane/eventloop"
	"github.com/solo-io/gloo/pkg/upstream-discovery"
	"github.com/solo-io/gloo/pkg/upstream-discovery/bootstrap"
	controlplane "github.com/solo-io/gloo/pkg/control-plane/bootstrap"
	"math"
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

func Run(runConfig RunConfig, store storage.Interface) error {
	if runConfig.ConfigDir == "" {
		var err error
		runConfig.ConfigDir, err = ioutil.TempDir("", "")
		if err != nil {
			return err
		}
		defer os.RemoveAll(runConfig.ConfigDir)
	}

	// set consul options from consul config
	consulCfg := api.DefaultConfig()
	cfg, err := consul.NewConsulConnectConfigFromEnv()
	if err != nil {
		return err
	}
	runConfig.Options.ConsulOptions.Token = cfg.Token()
	runConfig.Options.ConsulOptions.Address = consulCfg.Address
	runConfig.Options.ConsulOptions.Datacenter = consulCfg.Datacenter
	runConfig.Options.ConsulOptions.Scheme = consulCfg.Scheme
	if consulCfg.HttpAuth != nil {
		runConfig.Options.ConsulOptions.Username = consulCfg.HttpAuth.Username
		runConfig.Options.ConsulOptions.Password = consulCfg.HttpAuth.Password
	}

	// wrap the config store with our in-memory one
	store = localstorage.NewPartialInMemoryConfig(store)

	// create a secret client for in-memory certificates
	secrets := localstorage.NewInMemorySecrets()

	files, err := artifactstorage.Bootstrap(runConfig.Options)
	if err != nil {
		return pkgerrs.Wrap(err, "creating file storage client")
	}

	opts := controlplane.Options{
		Options: runConfig.Options,
		// TODO(ilackarms): change embedded gloo to not require ingress options
		IngressOptions: controlplane.IngressOptions{
			Port: math.MaxUint32,
			SecurePort: math.MaxUint32-1,
		},
	}
	controlPlane, err := eventloop.SetupWithStorage(opts, store, secrets, files, int(runConfig.GlooPort))
	if err != nil {
		return pkgerrs.Wrap(err, "creating control-plane event loop")
	}

	port := uint32(8500)
	addr := "127.0.0.1"

	addresparts := strings.Split(consulCfg.Address, ":")

	if len(addresparts) == 2 {
		addr = addresparts[0]
		port32, _ := strconv.Atoi(addresparts[1])
		port = uint32(port32)
	}

	log.Printf("creating config writer")

	rolename, configWriter := gloo.NewConfigWriter(store, cfg, gloo.ConsulInfo{
		ConsulHostname: addr,
		ConsulPort:     port,
		AuthorizePath:  "/v1/agent/connect/authorize",
		ConfigDir:      runConfig.ConfigDir,
	})

	ctx := context.Background()
	ctx, cancelTerm := cancelOnTerm(ctx)
	defer cancelTerm()

	//create stop channel from context
	stop := make(chan struct{})
	go func(){
		<-ctx.Done()
		close(stop)
	}()
	go controlPlane.Run(stop)
	go func() {
		opts := bootstrap.Options{
			Options: runConfig.Options,
			UpstreamDiscoveryOptions: bootstrap.UpstreamDiscoveryOptions{
				EnableDiscoveryForConsul: true,
			},
		}
		if err := upstreamdiscovery.Start(opts, store, stop); err != nil {
			log.Fatalf("failed to start upstream discovery: %v", err)
		}
	}()

	log.Printf("creating cert fetcher")
	cf, err := consul.NewCertificateFetcher(ctx, configWriter, cfg)
	if err != nil {
		return err
	}

	log.Printf("getting first copy of local certs")
	// we need one root cert and client cert to begin:
	rootcert := <-cf.RootCerts()
	leaftcert := <-cf.Certs()

	id := &envoycore.Node{
		Id:      rolename + "~" + getNodeName(),
		Cluster: cfg.ProxyId(),
	}

	e := envoy.NewEnvoy(runConfig.EnvoyPath, runConfig.GlooAddress, runConfig.GlooPort, runConfig.ConfigDir, id, secrets)
	envoyCfg := envoy.Config{
		LeafCert: leaftcert,
		RootCas:  rootcert,
	}

	log.Printf("writing envoy config")
	err = e.WriteConfig(envoyCfg)
	if err != nil {
		return errors.New("can't write config")
	}

	log.Printf("starting envoy config")
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
