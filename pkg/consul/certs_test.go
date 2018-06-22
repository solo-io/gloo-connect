package consul_test

import (
	"context"

	"github.com/hashicorp/consul/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/solo-io/gloo-consul-bridge/pkg/consul"
)

type fakeConfigWriter struct {
}

func (f *fakeConfigWriter) Write(cfg *api.ProxyInfo) error {
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
	rootschan   chan *api.RootsInfo
	leafchan    chan *api.LeafCertInfo
	pconfigchan chan *api.ProxyInfo
}

func generateQm() *api.QueryMeta {
	return &api.QueryMeta{
		LastIndex: 1,
	}
}

func (c *mockConnectClient) RootCerts(q *api.QueryOptions) (*api.RootsInfo, *api.QueryMeta, error) {
	return <-c.rootschan, generateQm(), nil
}

func (c *mockConnectClient) LeafCert(svcname string, q *api.QueryOptions) (*api.LeafCertInfo, *api.QueryMeta, error) {
	return <-c.leafchan, generateQm(), nil
}

func (c *mockConnectClient) ProxyConfig(proxyid string, q *api.QueryOptions) (*api.ProxyInfo, *api.QueryMeta, error) {
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
		singleRootsInfo *api.RootsInfo
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		mockClient = &mockConnectClient{
			rootschan:   make(chan *api.RootsInfo, 10),
			leafchan:    make(chan *api.LeafCertInfo, 10),
			pconfigchan: make(chan *api.ProxyInfo, 10),
		}

		cf, err := NewCertificateFetcherFromInterface(ctx, &fakeConfigWriter{}, &fakeConsulConnectConfig{}, mockClient)
		Expect(err).NotTo(HaveOccurred())
		certificateFetcher = cf

		singleRootsInfo = &api.RootsInfo{
			ActiveRootID: "123",
			Roots: []api.RootCA{{
				ID:       "123",
				RootCert: "123",
				Active:   true,
			}},
		}

	})

	AfterEach(func() {
		cancel()
	})

	Context("root certs", func() {

		It("should not get root cert when none arrives", func() {
			mockClient.rootschan <- &api.RootsInfo{ActiveRootID: "none"}
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
			secondInfo := &api.RootsInfo{
				ActiveRootID: "567",
				Roots: []api.RootCA{{
					ID:       "567",
					RootCert: "567",
					Active:   true,
				}},
			}
			mockClient.rootschan <- secondInfo
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
			Eventually(certificateFetcher.RootCerts()).Should(Receive())
		})

	})
})
