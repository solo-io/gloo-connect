package gloo

import (
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/vektah/gqlgen/neelance/errors"
	"github.com/hashicorp/consul/api"
	"sort"
	"github.com/solo-io/gloo-connect/pkg/gloo/connect"
)

type ProxyConfig struct {
	BindAddress         string     `json:"bind_address" mapstructure:"bind_address"`
	BindPort            uint       `json:"bind_port" mapstructure:"bind_port"`
	LocalServiceAddress string     `json:"local_service_address" mapstructure:"local_service_address"`
	Upstreams           []Upstream `json:"upstreams" mapstructure:"upstreams"`
}

type Upstream struct {
	DestinationName string `json:"destination_name" mapstructure:"destination_name"`
	DestinationType string `json:"destination_type" mapstructure:"destination_type"`
	LocalBindPort   uint32 `json:"local_bind_port" mapstructure:"local_bind_port"`
}

type ConsulInfo struct {
	// dns-resolvable hostname of the local consul agent
	ConsulHostname string
	// port of the local consul agent
	ConsulPort uint32
	// path where consul is serving Authorize requests
	AuthorizePath string
	// dir where gloo bridge config is stored
	ConfigDir string
}

type ConfigMerger struct {
	gloo   storage.Interface
	consul *api.Client
}

var _ storage.Interface = &ConfigMerger{}

func (s *ConfigMerger) V1() storage.V1 {
	return s
}

func (s *ConfigMerger) Register() error {
	return s.gloo.V1().Register()
}

func (s *ConfigMerger) Upstreams() storage.Upstreams {
	return s.gloo.V1().Upstreams()
}

func (s *ConfigMerger) VirtualServices() storage.VirtualServices {
	return s.gloo.V1().VirtualServices()
}

func (s *ConfigMerger) Roles() storage.Roles {
	return s
}

func (s *ConfigMerger) Create(*v1.Role) (*v1.Role, error) {
	return nil, errors.Errorf("Roles.Create is disabled for ConfigMerger")
}

func (s *ConfigMerger) Update(*v1.Role) (*v1.Role, error) {
	return nil, errors.Errorf("Roles.Update is disabled for ConfigMerger")
}

func (s *ConfigMerger) Delete(name string) error {
	return errors.Errorf("Roles.Delte is disabled for ConfigMerger")
}

func (s *ConfigMerger) Get(name string) (*v1.Role, error) {

}

func (s *ConfigMerger) List() ([]*v1.Role, error) {}

func (s *ConfigMerger) Watch(...storage.RoleEventHandler) (*storage.Watcher, error) {}

func convertProxyConfig(cfg *api.ConnectProxyConfig) *v1.Role {

}

func (cw *ConfigWriter) updateRole(role *v1.Role, pcfg *api.ConnectProxyConfig) (*v1.Role, error) {
	cfg, err := GetProxyConfig(pcfg)
	if err != nil {
		return nil, err
	}
	upstreams := cfg.Upstreams
	requiredListeners := 1 + len(upstreams)
	if len(role.Listeners) < requiredListeners {
		for i := len(role.Listeners); i <= requiredListeners; i++ {
			role.Listeners = append(role.Listeners, &v1.Listener{})
		}
	}
	syncInboundListener(role.Listeners[0], pcfg, cfg, cw.consulInfo)
	// sort upstreams for idempotency
	sort.SliceStable(upstreams, func(i, j int) bool {
		return upstreams[i].LocalBindPort < upstreams[j].LocalBindPort
	})
	for i, upstream := range cfg.Upstreams {
		syncOutboundListener(role.Listeners[i+1], upstream)
	}
	return role, nil
}

func inboundListener(pcfg *api.ConnectProxyConfig, cfg *ProxyConfig, consul ConsulInfo) *v1.Listener {
	return &v1.Listener{
		Name:        pcfg.ProxyServiceID + "-inbound",
		BindAddress: cfg.BindAddress,
		BindPort:    uint32(cfg.BindPort),
		Config: connect.EncodeListenerConfig(&connect.ListenerConfig{
			Config: &connect.ListenerConfig_Inbound{
				Inbound: &connect.InboundListenerConfig{
					LocalServiceName:    pcfg.TargetServiceName,
					LocalServiceAddress: cfg.LocalServiceAddress,
					AuthConfig: &connect.AuthConfig{
						Target:            pcfg.TargetServiceName,
						AuthorizeHostname: consul.ConsulHostname,
						AuthorizePort:     consul.ConsulPort,
						AuthorizePath:     consul.AuthorizePath,
					},
				},
			},
		}),
	}
	// TODO (ilackarms): RequestTimeout:
	inbound.AuthConfig = authConfig
	inboundConfig.Inbound = inbound
	listenerConfig.Config = inboundConfig
	connect.SetListenerConfig(listener, listenerConfig)
	caCert, privateKey, rootCa := secretPaths(consul.ConfigDir)
	listener.SslConfig = &v1.SSLConfig{
		SslSecrets: &v1.SSLConfig_SslFiles{
			SslFiles: &v1.SSLFiles{
				TlsCert: caCert,
				TlsKey:  privateKey,
				RootCa:  rootCa,
			},
		},
	}
}

func syncOutboundListener(listener *v1.Listener, upstream Upstream) {
	listener.Name = upstream.DestinationName + "-outbound"
	// TODO (ilackarms): support ipv6
	listener.BindAddress = "127.0.0.1"
	listener.BindPort = upstream.LocalBindPort
	listenerConfig, err := connect.DecodeListenerConfig(listener.Config)
	if err != nil || listenerConfig == nil {
		listenerConfig = &connect.ListenerConfig{}
	}
	if listenerConfig.Config == nil {
		listenerConfig.Config = &connect.ListenerConfig_Outbound{}
	}
	outboundConfig, ok := listenerConfig.Config.(*connect.ListenerConfig_Outbound)
	if !ok {
		outboundConfig = &connect.ListenerConfig_Outbound{}
	}
	outbound := outboundConfig.Outbound
	if outbound == nil {
		outbound = &connect.OutboundListenerConfig{}
	}
	outbound.DestinationConsulService = upstream.DestinationName
	outbound.DestinationConsulType = upstream.DestinationType
	outboundConfig.Outbound = outbound
	listenerConfig.Config = outboundConfig
	connect.SetListenerConfig(listener, listenerConfig)
}
