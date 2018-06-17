package consul

import (
	"errors"
	"os"
)

type ConsulConnectConfig interface {
	ProxyId() string
	Token() string
}

type consulConnectConfig struct {
	proxyId, token string
}

func (c *consulConnectConfig) ProxyId() string { return c.proxyId }
func (c *consulConnectConfig) Token() string   { return c.token }

func NewConsulConnectConfigFromEnv() (ConsulConnectConfig, error) {
	cfg := &consulConnectConfig{
		proxyId: os.Getenv("CONSUL_PROXY_ID"),
		token:   os.Getenv("CONSUL_PROXY_TOKEN"),
	}

	if cfg.proxyId == "" {
		return nil, errors.New("can't detect config from env")
	}

	return cfg, nil
}
