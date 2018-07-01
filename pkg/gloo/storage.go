package gloo

import (
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/hashicorp/consul/api"
	"sort"
	"github.com/solo-io/gloo-connect/pkg/gloo/connect"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"time"
	"github.com/solo-io/gloo/pkg/log"
	"github.com/solo-io/gloo-connect/pkg/consul"
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
	proxyId    string
	gloo       storage.Interface
	consul     consul.ConnectClient
	consulInfo ConsulInfo
}

var _ storage.Interface = &ConfigMerger{}

func NewConfigMerger(proxyId string, gloo storage.Interface, consul consul.ConnectClient, info ConsulInfo) *ConfigMerger {
	return &ConfigMerger{
		proxyId:    proxyId,
		gloo:       gloo,
		consul:     consul,
		consulInfo: info,
	}
}

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
	return nil, errors.Errorf("Roles.Get is disabled for ConfigMerger")
}

func (s *ConfigMerger) List() ([]*v1.Role, error) {
	return nil, errors.Errorf("Roles.Delte is disabled for ConfigMerger")
}

func (s *ConfigMerger) Watch(handlers ...storage.RoleEventHandler) (*storage.Watcher, error) {
	configs := s.watchConnectConfigs()

	var isUpdate bool

	return storage.NewWatcher(func(stop <-chan struct{}, errs chan error) {
		for {
			select {
			case connectConfig := <-configs:
				role, err := s.role(connectConfig)
				if err != nil {
					log.Warnf("error syncing with consul connect config: %v", err)
					continue
				}
				for _, h := range handlers {
					if !isUpdate {
						h.OnAdd([]*v1.Role{role}, role)
					} else {
						h.OnUpdate([]*v1.Role{role}, role)
					}
				}
				isUpdate = true
			case err := <-errs:
				log.Warnf("failed to start watcher to: %v", err)
				return
			case <-stop:
				return
			}
		}
	}), nil
}

func (s *ConfigMerger) watchConnectConfigs() <-chan *api.ConnectProxyConfig {
	configs := make(chan *api.ConnectProxyConfig)

	go func() {
		var opts *api.QueryOptions
		for {
			connectConfig, query, err := s.consul.ConnectProxyConfig(s.proxyId, opts)
			if err != nil {
				log.Printf("errror watching connect config: %v", err)
				time.Sleep(time.Second)
				continue
			}
			opts = &api.QueryOptions{
				WaitIndex: query.LastIndex,
			}
			configs <- connectConfig
		}
	}()

	return configs
}

func (s *ConfigMerger) role(connectConfig *api.ConnectProxyConfig) (*v1.Role, error) {
	proxyConfig, err := parseProxyConfig(connectConfig)
	if err != nil {
		return nil, err
	}
	listeners := []*v1.Listener{inboundListener(connectConfig, proxyConfig, s.consulInfo)}
	for _, upstream := range proxyConfig.Upstreams {
		listeners = append(listeners, outboundListener(upstream))
	}

	// sort listeners for idempotency
	sort.SliceStable(listeners, func(i, j int) bool {
		return listeners[i].BindPort < listeners[j].BindPort
	})

	return &v1.Role{
		Name:      s.proxyId,
		Listeners: listeners,
	}, nil
}

func inboundListener(connectConfig *api.ConnectProxyConfig, proxyConfig *ProxyConfig, consul ConsulInfo) *v1.Listener {
	return &v1.Listener{
		Name:        connectConfig.ProxyServiceID + "-inbound",
		BindAddress: proxyConfig.BindAddress,
		BindPort:    uint32(proxyConfig.BindPort),
		Config: connect.EncodeListenerConfig(&connect.ListenerConfig{
			Config: &connect.ListenerConfig_Inbound{
				Inbound: &connect.InboundListenerConfig{
					LocalServiceName:    connectConfig.TargetServiceName,
					LocalServiceAddress: proxyConfig.LocalServiceAddress,
					AuthConfig: &connect.AuthConfig{
						Target:            connectConfig.TargetServiceName,
						AuthorizeHostname: consul.ConsulHostname,
						AuthorizePort:     consul.ConsulPort,
						AuthorizePath:     consul.AuthorizePath,
					},
				},
			},
		}),
		SslConfig: nil, //TODO
	}
}

func outboundListener(upstream Upstream) *v1.Listener {
	return &v1.Listener{
		Name:        upstream.DestinationName + "-outbound",
		BindAddress: "127.0.0.1",
		BindPort:    upstream.LocalBindPort,
		Config: connect.EncodeListenerConfig(&connect.ListenerConfig{
			Config: &connect.ListenerConfig_Outbound{
				Outbound: &connect.OutboundListenerConfig{
					DestinationConsulService: upstream.DestinationName,
					DestinationConsulType:    upstream.DestinationType,
				},
			},
		}),
	}
}

func parseProxyConfig(connectConfig *api.ConnectProxyConfig) (*ProxyConfig, error) {
	proxyConfig := new(ProxyConfig)
	err := mapstructure.Decode(connectConfig.Config, proxyConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding map[string]interface{} as proxy config")
	}
	return proxyConfig, nil
}
