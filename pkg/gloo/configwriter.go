package gloo

import (
	"github.com/hashicorp/consul/api"
	"github.com/solo-io/consul-gloo-bridge/pkg/consul"
)

type GlooConfigWriter struct {
}

func (g *GlooConfigWriter) Write(cfg *api.ProxyInfo) error {
	panic("TODO")
}

func NewConfigWriter(cfg consul.ConsulConnectConfig) (string, consul.ConfigWriter) {
	panic("TODO: return (rolename, configobj)")
}
