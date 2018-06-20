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

var _ consul.ConfigWriter = &GlooConfigWriter{}
