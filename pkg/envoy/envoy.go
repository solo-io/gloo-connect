package envoy

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/solo-io/consul-gloo-bridge/pkg/types"
)

type Config struct {
	LeafCert types.CertificateAndKey
	RootCas  types.Certificates
}

type Envoy interface {
	Run()
	Exit()

	WriteConfig(cfg Config) error
	Reload() error
}

type EnvoyInstance struct {
	Done    <-chan error
	Process *os.Process
}

type envoy struct {
	restartEpoch uint
	glooAddress  string
	glooPort     uint
	configDir    string
	id           *envoycore.Node
	envoyBin     string

	children []*EnvoyInstance

	configChanged chan struct{}
	doneInstances chan *EnvoyInstance
}

func NewEnvoy(envoyBin string, glooAddress string, glooPort uint, configDir string, id *envoycore.Node) Envoy {
	if envoyBin == "" {
		envoyBin, _ = exec.LookPath("envoy")
	}
	return &envoy{
		glooAddress: glooAddress,
		glooPort:    glooPort,
		configDir:   configDir,
		id:          id,
		envoyBin:    envoyBin,

		configChanged: make(chan struct{}, 10),
		doneInstances: make(chan *EnvoyInstance),
	}
}

func (e *envoy) Run() {
	// start envoy one time at least to make sure we have children
	<-e.configChanged
	e.startEnvoyAndWatchit()
	for {
		if len(e.children) == 0 {
			return
		}
		select {
		case ei := <-e.doneInstances:
			e.remove(ei)
		case <-e.configChanged:
			e.startEnvoyAndWatchit()
		}
	}
}

func (e *envoy) remove(ei *EnvoyInstance) {
	for i := range e.children {
		if ei == e.children[i] {
			e.children = append(e.children[:i], e.children[i+1:]...)
			return
		}
	}
}

func (e *envoy) WriteConfig(cfg Config) error {
	err := ioutil.WriteFile(filepath.Join(e.configDir, "rootcas.crt"), []byte(cfg.RootCas.String()), 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(e.configDir, "leaf.crt"), []byte(cfg.LeafCert.Certificate), 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(e.configDir, "leaf.key"), []byte(cfg.LeafCert.PrivateKey), 0600)
	if err != nil {
		return err
	}

	// TODO: write the envoy config file it self?
	bootconfig := e.getBootstrapConfig()
	jsonpbMarshaler := &jsonpb.Marshaler{OrigName: true}

	var buf bytes.Buffer
	err = jsonpbMarshaler.Marshal(&buf, &bootconfig)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(e.getEnvoyConfigPath(), buf.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (e *envoy) getEnvoyConfigPath() string {
	return filepath.Join(e.configDir, "envoy.json")
}

func (e *envoy) getBootstrapConfig() envoybootstrap.Bootstrap {
	var bootstrap envoybootstrap.Bootstrap

	const glooClusterName = "xds_cluster"

	bootstrap.Node = e.id

	// get gloo xds
	bootstrap.DynamicResources = &envoybootstrap.Bootstrap_DynamicResources{
		CdsConfig: &envoycore.ConfigSource{
			ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
				Ads: &envoycore.AggregatedConfigSource{},
			},
		},
		LdsConfig: &envoycore.ConfigSource{
			ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
				Ads: &envoycore.AggregatedConfigSource{},
			},
		},
		AdsConfig: &envoycore.ApiConfigSource{
			ApiType: envoycore.ApiConfigSource_GRPC,
			GrpcServices: []*envoycore.GrpcService{{
				TargetSpecifier: &envoycore.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &envoycore.GrpcService_EnvoyGrpc{
						ClusterName: glooClusterName,
					},
				},
			}},
		},
	}
	bootstrap.StaticResources = &envoybootstrap.Bootstrap_StaticResources{
		Clusters: []envoyapi.Cluster{{
			Name:                 glooClusterName,
			ConnectTimeout:       5 * time.Second,
			Http2ProtocolOptions: &envoycore.Http2ProtocolOptions{},
			Type:                 envoyapi.Cluster_STRICT_DNS,
			Hosts: []*envoycore.Address{{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Protocol: envoycore.TCP,
						Address:  e.glooAddress,
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: uint32(e.glooPort),
						},
					},
				},
			}},
		}},
	}

	bootstrap.Admin = envoybootstrap.Admin{
		AccessLogPath: "/dev/stderr",
		Address: envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Protocol: envoycore.TCP,
					Address:  "127.0.0.1",
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: 0,
					},
				},
			},
		},
	}

	return bootstrap
}

func (e *envoy) Reload() error {
	e.configChanged <- struct{}{}
	return nil
}

func (e *envoy) startEnvoyAndWatchit() error {
	ei, err := e.startEnvoy()
	if err != nil {
		return err
	}

	go func() {
		// TODO: log errors
		<-ei.Done
		e.doneInstances <- ei
	}()

	e.children = append(e.children, ei)
	return nil
}

func (e *envoy) Exit() {
	for _, c := range e.children {
		c.Process.Signal(syscall.SIGTERM)
	}
}

func (e *envoy) startEnvoy() (*EnvoyInstance, error) {
	// start new envoy and pass the restart epoch

	// TODO add config file
	envoyCommand := exec.Command(e.envoyBin, "--restart-epoch", fmt.Sprintf("%d", e.restartEpoch), "--config-path", e.getEnvoyConfigPath(), "--v2-config-only")
	err := envoyCommand.Start()
	if err != nil {
		return nil, err
	}

	envoiddied := make(chan error, 1)

	go func() {
		defer close(envoiddied)
		err := envoyCommand.Wait()
		if err != nil {
			// envoy died prematuraly :(
			envoiddied <- err
		}
	}()

	// wait for 5 seconds to see if envoy didn't die (i.e. hot restarting failed)
	select {
	case err := <-envoiddied:
		if err == nil {
			return nil, errors.New("envoy died prematurly")
		}
		return nil, err
	case <-time.After(5 * time.Second):
	}

	e.restartEpoch++
	return &EnvoyInstance{Done: envoiddied, Process: envoyCommand.Process}, nil
}

/*

In here we will:
Create envoy config to talk with gloo; initially un encrypted.

in write config, write files to
/*
/somedir/rootca/proxyid/rootcas.crt
/somedir/rootca/proxyid/leaf.crt
/somedir/rootca/proxyid/leaf.key


*/
