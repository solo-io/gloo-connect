package consul

import (
	"context"
	"reflect"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/solo-io/consul-gloo-bridge/pkg/types"
)

type ConfigWriter interface {
	Write(cfg *api.ProxyInfo) error
}

type ConnectClient interface {
	RootCerts(q *api.QueryOptions) (*api.RootsInfo, *api.QueryMeta, error)
	LeafCert(svcname string, q *api.QueryOptions) (*api.LeafCertInfo, *api.QueryMeta, error)
	ProxyConfig(proxyid string, q *api.QueryOptions) (*api.ProxyInfo, *api.QueryMeta, error)
}

type CertificateFetcher interface {
	Certs() <-chan types.CertificateAndKey
	RootCerts() <-chan types.Certificates
}

type certificateFetcher struct {
	c ConnectClient

	certs     chan types.CertificateAndKey
	rootCerts chan types.Certificates

	configWriter ConfigWriter
}

func (c *certificateFetcher) Certs() <-chan types.CertificateAndKey {
	return c.certs
}

func (c *certificateFetcher) RootCerts() <-chan types.Certificates {
	return c.rootCerts
}

func NewCertificateFetcher(ctx context.Context, configWriter ConfigWriter, cfg ConsulConnectConfig) (CertificateFetcher, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Token = cfg.Token()
	client, err := api.NewClient(consulConfig)

	if err != nil {
		return nil, err
	}
	return NewCertificateFetcherFromInterface(ctx, configWriter, cfg, api.NewAgentConnect(client))
}

func NewCertificateFetcherFromInterface(ctx context.Context, configWriter ConfigWriter, cfg ConsulConnectConfig, client ConnectClient) (CertificateFetcher, error) {

	c := &certificateFetcher{
		certs:     make(chan types.CertificateAndKey),
		rootCerts: make(chan types.Certificates),

		configWriter: configWriter,
	}
	c.c = client

	go c.getRoots(ctx)
	go c.getProxyConfig(ctx, cfg.ProxyId())

	return c, nil
}

func (c *certificateFetcher) getRoots(ctx context.Context) {

	var q *api.QueryOptions
	var certs types.Certificates

	for {
		q = q.WithContext(ctx)
		info, query, err := c.c.RootCerts(q)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// TODO: log error...
			time.Sleep(time.Second)
			continue
		}
		q = &api.QueryOptions{
			WaitIndex: query.LastIndex,
		}
		var newCerts types.Certificates
		for _, r := range info.Roots {
			if r.Active {
				newCerts = append(newCerts, types.Certificate(r.RootCert))
			}
		}
		if len(newCerts) != 0 {
			if !reflect.DeepEqual(newCerts, certs) {
				certs = newCerts
				c.rootCerts <- certs
			}
		}
	}
}

func (c *certificateFetcher) getProxyConfig(ctx context.Context, proxyid string) {
	var q *api.QueryOptions
	var leafStarted bool
	for {
		q = q.WithContext(ctx)
		proxyinfo, query, err := c.c.ProxyConfig(proxyid, q)

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// TODO: log error...
			time.Sleep(time.Second)
			continue
		}
		q = &api.QueryOptions{
			WaitIndex: query.LastIndex,
		}
		c.configWriter.Write(proxyinfo)
		if !leafStarted {
			go c.getLeaf(ctx, proxyinfo.TargetServiceName)
			leafStarted = true
		}
		if query.LastIndex == 0 {
			// if this is not a blocking query, exit
			return
		}
	}
}

func (c *certificateFetcher) getLeaf(ctx context.Context, service string) {
	var q *api.QueryOptions
	for {
		q = q.WithContext(ctx)
		info, query, err := c.c.LeafCert(service, q)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// TODO: log error...
			time.Sleep(time.Second)
			continue
		}
		q = &api.QueryOptions{
			WaitIndex: query.LastIndex,
		}

		c.certs <- types.CertificateAndKey{
			Certificate: types.Certificate(info.CertPEM),
			PrivateKey:  types.PrivateKey(info.PrivateKeyPEM),
		}
	}
}
