package envoy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/solo-io/gloo/pkg/log"
)

type Config struct {
}

type Envoy interface {
	Run(context.Context) error
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
	glooAddress  net.Addr
	glooPort     uint
	id           *envoycore.Node
	envoyBin     string

	children []*EnvoyInstance

	configChanged chan struct{}
	doneInstances chan *EnvoyInstance

	cfg string
}

func NewEnvoy(envoyBin string, glooAddress net.Addr, id *envoycore.Node) Envoy {
	if envoyBin == "" {
		envoyBin, _ = exec.LookPath("envoy")
	}
	return &envoy{
		glooAddress: glooAddress,
		id:          id,
		envoyBin:    envoyBin,

		configChanged: make(chan struct{}, 10),
		doneInstances: make(chan *EnvoyInstance),
	}
}

func (e *envoy) Run(ctx context.Context) error {
	// start envoy one time at least to make sure we have children
	<-e.configChanged
	if err := e.startEnvoyAndWatchit(); err != nil {
		return err
	}
	for {
		if len(e.children) == 0 {
			return nil
		}
		select {
		case ei := <-e.doneInstances:
			e.remove(ei)
		case <-e.configChanged:
			if err := e.startEnvoyAndWatchit(); err != nil {
				return err
			}
		case <-ctx.Done():
			e.Exit()
			return nil
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

	// TODO: write the envoy config file it self?
	bootconfig, err := e.getBootstrapConfig()
	if err != nil {
		return err
	}
	jsonpbMarshaler := &jsonpb.Marshaler{OrigName: true}

	var buf bytes.Buffer
	err = jsonpbMarshaler.Marshal(&buf, &bootconfig)
	if err != nil {
		return err
	}

	e.cfg = buf.String()

	return nil
}

func (e *envoy) getBootstrapConfig() (envoybootstrap.Bootstrap, error) {
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
	clusterType, addr, err := e.getAddress()
	if err != nil {
		return bootstrap, err
	}

	bootstrap.StaticResources = &envoybootstrap.Bootstrap_StaticResources{
		Clusters: []envoyapi.Cluster{{
			Name:                 glooClusterName,
			ConnectTimeout:       5 * time.Second,
			Http2ProtocolOptions: &envoycore.Http2ProtocolOptions{},
			Type:                 clusterType,
			Hosts:                []*envoycore.Address{addr},
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

	return bootstrap, nil
}

func (e *envoy) getAddress() (envoyapi.Cluster_DiscoveryType, *envoycore.Address, error) {
	switch e.glooAddress.Network() {
	case "tcp":
		host, port, err := net.SplitHostPort(e.glooAddress.String())
		if err != nil {
			return envoyapi.Cluster_STATIC, nil, err
		}
		intport, err := strconv.Atoi(port)
		if err != nil {
			return envoyapi.Cluster_STATIC, nil, err
		}
		tcpaddr := &envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Protocol: envoycore.TCP,
					Address:  host,
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: uint32(intport),
					},
				},
			},
		}
		return envoyapi.Cluster_STRICT_DNS, tcpaddr, nil
	case "unix":
		unixaddr := &envoycore.Address{
			Address: &envoycore.Address_Pipe{
				Pipe: &envoycore.Pipe{
					Path: e.glooAddress.String(),
				},
			},
		}
		return envoyapi.Cluster_STATIC, unixaddr, nil
	}
	return envoyapi.Cluster_STATIC, nil, errors.New("unsupported address")
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

	e.children = append(e.children, ei)

	go func() {
		// TODO: log errors
		<-ei.Done
		e.doneInstances <- ei
	}()

	return nil
}

func (e *envoy) Exit() {
	for _, c := range e.children {
		c.Process.Signal(syscall.SIGTERM)
	}
}

func (e *envoy) startEnvoy() (*EnvoyInstance, error) {
	// start new envoy and pass the restart epoch
	envoyCommand := exec.Command(e.envoyBin, "--restart-epoch", fmt.Sprintf("%d", e.restartEpoch), "--config-yaml", e.cfg, "--v2-config-only")
	envoyCommand.Stderr = os.Stderr
	envoyCommand.Stdout = os.Stderr
	err := envoyCommand.Start()
	if err != nil {
		return nil, err
	}

	log.Printf("running envoy with cmd %v", envoyCommand.Args)

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
			return nil, errors.New("envoy died prematurely")
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
