package gloo

import (
	"sort"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/hashicorp/consul/api"
	"github.com/solo-io/consul-gloo-bridge/pkg/consul"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/solo-io/gloo/pkg/plugins/connect"
	"github.com/solo-io/gloo/pkg/storage"
)

type ConfigWriter struct {
	roleName   string
	gloo       storage.Interface
	consulInfo ConsulInfo
}

type ConsulInfo struct {
	// dns-resolvable hostname of the local consul agent
	ConsulHostname string
	// port of the local consul agent
	ConsulPort uint32
	// path where consul is serving Authorize requests
	AuthorizePath string
}

func (cw *ConfigWriter) Write(cfg *api.ProxyInfo) error {
	return cw.syncRole(cfg)
}

func NewConfigWriter(gloo storage.Interface, cfg consul.ConsulConnectConfig, consulInfo ConsulInfo) (string, consul.ConfigWriter) {
	roleName := cfg.ProxyId()
	return roleName, &ConfigWriter{
		roleName:   roleName,
		gloo:       gloo,
		consulInfo: consulInfo,
	}
}

func (cw *ConfigWriter) syncRole(cfg *api.ProxyInfo) error {
	role, err := cw.gloo.V1().Roles().Get(cw.roleName)
	if err != nil {
		role, err = cw.gloo.V1().Roles().Create(&v1.Role{
			Name: cw.roleName,
		})
		if err != nil {
			return err
		}
	}
	// clone the role, use this to determine if a storage write is necessary
	updatedRole := cw.updateRole(proto.Clone(role).(*v1.Role), cfg)
	if role.Equal(updatedRole) {
		return nil
	}
	if _, err := cw.gloo.V1().Roles().Update(updatedRole); err != nil {
		return errors.Wrapf(err, "updating role %v", role.Name)
	}
	return nil
}

func (cw *ConfigWriter) updateRole(role *v1.Role, cfg *api.ProxyInfo) *v1.Role {
	upstreams := cfg.Config.Upstreams
	requiredListeners := 1 + len(upstreams)
	if len(role.Listeners) < requiredListeners {
		for i := len(role.Listeners); i <= requiredListeners; i++ {
			role.Listeners = append(role.Listeners, &v1.Listener{})
		}
	}
	syncInboundListener(role.Listeners[0], cfg, cw.consulInfo.ConsulHostname, cw.consulInfo.ConsulPort, cw.consulInfo.AuthorizePath)
	// sort upstreams for idempotency
	sort.SliceStable(upstreams, func(i, j int) bool {
		return upstreams[i].LocalBindPort < upstreams[j].LocalBindPort
	})
	for i, upstream := range cfg.Config.Upstreams {
		syncOutboundListener(role.Listeners[i+1], upstream)
	}
	return role
}

func syncInboundListener(listener *v1.Listener, cfg *api.ProxyInfo, consulHostname string, consulPort uint32, authorizePath string) {
	listener.Name = cfg.ProxyServiceID + "-inbound"
	listener.BindAddress = cfg.Config.BindAddress
	listener.BindPort = uint32(cfg.Config.BindPort)
	listenerConfig, err := connect.DecodeListenerConfig(listener.Config)
	if err != nil || listenerConfig == nil {
		listenerConfig = &connect.ListenerConfig{}
	}
	if listenerConfig.Config == nil {
		listenerConfig.Config = &connect.ListenerConfig_Inbound{}
	}
	inboundConfig, ok := listenerConfig.Config.(*connect.ListenerConfig_Inbound)
	if !ok {
		inboundConfig = &connect.ListenerConfig_Inbound{}
	}
	inbound := inboundConfig.Inbound
	if inbound == nil {
		inbound = &connect.InboundListenerConfig{}
	}
	authConfig := inbound.AuthConfig
	if authConfig == nil {
		authConfig = &connect.AuthConfig{}
	}
	authConfig.Target = cfg.TargetServiceName
	authConfig.AuthorizeHostname = consulHostname
	authConfig.AuthorizePort = consulPort
	authConfig.AuthorizePath = authorizePath
	// TODO (ilackarms): RequestTimeout:
	inbound.AuthConfig = authConfig
	inboundConfig.Inbound = inbound
	listenerConfig.Config = inboundConfig
}

func syncOutboundListener(listener *v1.Listener, upstream api.Upstream) {
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
}
