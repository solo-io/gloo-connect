package consul_test

import (
	"context"

	"github.com/hashicorp/consul/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/solo-io/gloo-connect/pkg/consul"
)

type fakeConfigWriter struct {
}

func (f *fakeConfigWriter) Write(cfg *api.ConnectProxyConfig) error {
	return nil
}

type fakeConsulConnectConfig struct {
}

func (f *fakeConsulConnectConfig) ProxyId() string {
	return "123"
}

func (f *fakeConsulConnectConfig) Token() string {
	return "123"
}

type mockConnectClient struct {
	rootschan   chan *api.CARootList
	leafchan    chan *api.LeafCert
	pconfigchan chan *api.ConnectProxyConfig
}

func generateQm() *api.QueryMeta {
	return &api.QueryMeta{
		LastIndex: 1,
	}
}

func (c *mockConnectClient) ConnectCARoots(q *api.QueryOptions) (*api.CARootList, *api.QueryMeta, error) {
	return <-c.rootschan, generateQm(), nil
}

func (c *mockConnectClient) ConnectCALeaf(svcname string, q *api.QueryOptions) (*api.LeafCert, *api.QueryMeta, error) {
	return <-c.leafchan, generateQm(), nil
}

func (c *mockConnectClient) ConnectProxyConfig(proxyid string, q *api.QueryOptions) (*api.ConnectProxyConfig, *api.QueryMeta, error) {
	return <-c.pconfigchan, generateQm(), nil
}

var _ = Describe("Certs", func() {
	var (
		ctx                context.Context
		cancel             context.CancelFunc
		mockClient         *mockConnectClient
		certificateFetcher CertificateFetcher
	)

	var (
		singleRootsInfo *api.CARootList
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		mockClient = &mockConnectClient{
			rootschan:   make(chan *api.CARootList, 10),
			leafchan:    make(chan *api.LeafCert, 10),
			pconfigchan: make(chan *api.ConnectProxyConfig, 10),
		}

		cf, err := NewCertificateFetcherFromInterface(ctx, &fakeConfigWriter{}, &fakeConsulConnectConfig{}, mockClient)
		Expect(err).NotTo(HaveOccurred())
		certificateFetcher = cf

		singleRootsInfo = &api.CARootList{
			ActiveRootID: "123",
			Roots: []*api.CARoot{{
				ID:          "123",
				RootCertPEM: "123",
				Active:      true,
			}},
		}

	})

	AfterEach(func() {
		cancel()
	})

	Context("root certs", func() {

		It("should not get root cert when none arrives", func() {
			mockClient.rootschan <- &api.CARootList{ActiveRootID: "none"}
			Consistently(certificateFetcher.RootCerts()).ShouldNot(Receive())
		})

		It("should get root cert when it arrives", func() {
			mockClient.rootschan <- singleRootsInfo
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
		})

		It("should get only get the same root cert once", func() {
			mockClient.rootschan <- singleRootsInfo
			mockClient.rootschan <- singleRootsInfo
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
			Consistently(certificateFetcher.RootCerts()).ShouldNot(Receive())
		})

		It("should get only get new certs", func() {
			mockClient.rootschan <- singleRootsInfo
			mockClient.rootschan <- singleRootsInfo
			secondInfo := &api.CARootList{
				ActiveRootID: "567",
				Roots: []*api.CARoot{{
					ID:          "567",
					RootCertPEM: "567",
					Active:      true,
				}},
			}
			mockClient.rootschan <- secondInfo
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
		})

	})
})
