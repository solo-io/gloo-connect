package consul

import (
	"errors"
	"os"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
)

type ConsulConnectConfig interface {
	ProxyId() string
	Token() string
}

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

type consulConnectConfig struct {
	proxyId, token string
}

func (c *consulConnectConfig) ProxyId() string { return c.proxyId }
func (c *consulConnectConfig) Token() string   { return c.token }

func NewConsulConnectConfigFromEnv() (ConsulConnectConfig, error) {
	cfg := &consulConnectConfig{
		proxyId: os.Getenv("CONNECT_PROXY_ID"),
		token:   os.Getenv("CONNECT_PROXY_TOKEN"),
	}

	if cfg.proxyId == "" {
		return nil, errors.New("can't detect config from env")
	}

	return cfg, nil
}

func GetProxyConfig(pcfg *api.ConnectProxyConfig) (*ProxyConfig, error) {
	cfg := new(ProxyConfig)

	err := mapstructure.Decode(pcfg.Config, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
