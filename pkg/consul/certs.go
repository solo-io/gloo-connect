package consul

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/solo-io/consul-gloo-bridge/pkg/types"
)

type CertificateFetcher interface {
	Certs() <-chan types.CertificateAndKey
	RootCerts() <-chan types.Certificates
}

type certificateFetcher struct {
	consulConfig *api.Config
	c            *api.Client

	certs     chan types.CertificateAndKey
	rootCerts chan types.Certificates
}

func (c *certificateFetcher) Certs() <-chan types.CertificateAndKey {
	return c.certs
}
func (c *certificateFetcher) RootCerts() <-chan types.Certificates {
	return c.rootCerts
}

func NewCertificateFetcher(ctx context.Context, cfg ConsulConnectConfig) (CertificateFetcher, error) {
	c := &certificateFetcher{
		certs:     make(chan types.CertificateAndKey),
		rootCerts: make(chan types.Certificates),
	}
	c.consulConfig = api.DefaultConfig()
	c.consulConfig.Token = cfg.Token()
	var err error
	c.c, err = api.NewClient(c.consulConfig)
	if err != nil {
		return nil, err
	}

	go c.getRoots(ctx)
	go c.getProxyConfig(ctx, cfg.ProxyId())

	return c, nil
}

func (c *certificateFetcher) getRoots(ctx context.Context) {
	client := api.NewAgentConnect(c.c)

	var q *api.QueryOptions
	for {
		q = q.WithContext(ctx)
		info, query, err := client.RootCerts(q)
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

		var certs types.Certificates
		for _, r := range info.Roots {
			if r.Active {
				certs = append(certs, types.Certificate(r.RootCert))
			}
		}
		c.rootCerts <- certs
	}
}

func (c *certificateFetcher) getProxyConfig(ctx context.Context, proxyid string) {
	client := api.NewAgentConnect(c.c)
	var q *api.QueryOptions
	var leafStarted bool
	for {
		q = q.WithContext(ctx)
		proxyinfo, query, err := client.ProxyConfig(proxyid, q)

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
		if !leafStarted {
			go c.getLeaf(ctx, proxyinfo.TargetServiceName)
			leafStarted = true
		}
		// TODO: upstream config potentially update
	}
}

func (c *certificateFetcher) getLeaf(ctx context.Context, service string) {
	client := api.NewAgentConnect(c.c)
	var q *api.QueryOptions
	for {
		q = q.WithContext(ctx)
		info, query, err := client.LeafCert(service, q)
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
