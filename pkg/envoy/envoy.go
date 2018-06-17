package envoy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
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
	WriteConfig(cfg Config)
	HotRestart()
}
type envoy struct {
	restartEpoch uint
	configDir    string
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

	err = ioutil.WriteFile(filepath.Join(e.configDir, "envoy.json"), buf.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (e *envoy) getBootstrapConfig() envoybootstrap.Bootstrap {
	var bootstrap envoybootstrap.Bootstrap

	const glooClusterName = "xds_cluster"

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
			Type:                 envoyapi.Cluster_STATIC,
			Hosts: []*envoycore.Address{{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Protocol: envoycore.TCP,
						// TODO
						Address: "TODO",
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: 0,
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

	panic("TODO")
	return bootstrap
}

func (e *envoy) HotRestart() {
	e.startEnvoy()
}

func (e *envoy) startEnvoy() error {
	// start new envoy and pass the restart epoch

	// TODO add config file
	envoyCommand := exec.Command("envoy", "--restart-epoch", fmt.Sprintf("%d", e.restartEpoch))
	err := envoyCommand.Start()
	if err != nil {
		return err
	}

	e.restartEpoch++
	return nil
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
