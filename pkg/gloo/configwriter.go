package gloo

import (
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/hashicorp/consul/api"
	"github.com/solo-io/gloo-connect/pkg/consul"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/solo-io/gloo/pkg/log"
	"github.com/solo-io/gloo/pkg/plugins/connect"
	"github.com/solo-io/gloo/pkg/storage"
)

const CertitificateSecretName = "certificates"

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
	// dir where gloo bridge config is stored
	ConfigDir string
}

func (cw *ConfigWriter) Write(cfg *api.ConnectProxyConfig) error {
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

func (cw *ConfigWriter) syncRole(cfg *api.ConnectProxyConfig) error {
	log.Printf("syncing role %s", cw.roleName)
	defer log.Printf("syncing role - done")

	role, err := cw.gloo.V1().Roles().Get(cw.roleName)
	if err != nil {
		role, err = cw.gloo.V1().Roles().Create(&v1.Role{
			Name: cw.roleName,
		})
		if err != nil {
			log.Warnf("error creating role: %v", err)
			return err
		}
	}
	log.Printf("retrieved existing role %v", role)

	// clone the role, use this to determine if a storage write is necessary
	updatedRole, err := cw.updateRole(proto.Clone(role).(*v1.Role), cfg)
	if err != nil {
		log.Warnf("error updating role: %v", err)
		return err
	}
	if role.Equal(updatedRole) {
		log.Printf("role is up to date; nothing to update")
		return nil
	}
	if _, err := cw.gloo.V1().Roles().Update(updatedRole); err != nil {
		err = errors.Wrapf(err, "updating role %v", role.Name)
		log.Warnf("error updating role: %v", err)
		return err
	}
	return nil
}

func (cw *ConfigWriter) updateRole(role *v1.Role, pcfg *api.ConnectProxyConfig) (*v1.Role, error) {
	cfg, err := consul.GetProxyConfig(pcfg)
	if err != nil {
		return nil, err
	}
	upstreams := cfg.Upstreams
	// one extra for the inbound listener
	// TODO(ilackarms): support client-only services (no listener)
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
		syncOutboundListener(role.Listeners[i+1], pcfg.TargetServiceName, upstream)
	}
	return role, nil
}

func syncInboundListener(listener *v1.Listener, pcfg *api.ConnectProxyConfig, cfg *consul.ProxyConfig, consulInfo ConsulInfo) {
	listener.Name = pcfg.ProxyServiceID + "-inbound"
	listener.BindAddress = cfg.BindAddress
	listener.BindPort = uint32(cfg.BindPort)
	listener.Labels = map[string]string{
		"service": pcfg.TargetServiceName,
		"inbound": "true",
	}
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
	inbound.LocalServiceName = pcfg.TargetServiceName
	inbound.LocalServiceAddress = cfg.LocalServiceAddress
	authConfig := inbound.AuthConfig
	if authConfig == nil {
		authConfig = &connect.AuthConfig{}
	}
	authConfig.Target = pcfg.TargetServiceName
	authConfig.AuthorizeHostname = consulInfo.ConsulHostname
	authConfig.AuthorizePort = consulInfo.ConsulPort
	authConfig.AuthorizePath = consulInfo.AuthorizePath
	// TODO (ilackarms): RequestTimeout:
	inbound.AuthConfig = authConfig
	inboundConfig.Inbound = inbound
	listenerConfig.Config = inboundConfig
	connect.SetListenerConfig(listener, listenerConfig)
	listener.SslConfig = &v1.SSLConfig{
		SslSecrets: &v1.SSLConfig_SecretRef{
			SecretRef: CertitificateSecretName,
		},
	}
}

func syncOutboundListener(listener *v1.Listener, targetServiceName string, upstream consul.Upstream) {
	listener.Name = upstream.DestinationName + "-outbound"
	// TODO (ilackarms): support ipv6
	listener.BindAddress = "127.0.0.1"
	listener.BindPort = upstream.LocalBindPort
	listener.Labels = map[string]string{
		"service":     targetServiceName,
		"destination": upstream.DestinationName,
	}
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
