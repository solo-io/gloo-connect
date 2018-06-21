package e2e_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	// . "github.com/solo-io/consul-gloo-bridge/e2e"
	"github.com/hashicorp/consul/api"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoyv2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache"

	xds "github.com/envoyproxy/go-control-plane/pkg/server"

	"google.golang.org/grpc"
)

var _ = Describe("ConsulConnect", func() {

	var tmpdir string
	var consulConfigDir string
	var consulSession *gexec.Session
	var pathToGlooBridge string

	BeforeSuite(func() {
		var err error
		pathToGlooBridge, err = gexec.Build("github.com/solo-io/consul-gloo-bridge/cmd")
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		envoypath := os.Getenv("ENVOY_PATH")
		Expect(envoypath).ToNot(BeEmpty())
		// generate the template
		svctemplate, err := ioutil.ReadFile("service.json.template")
		Expect(err).NotTo(HaveOccurred())

		tmpdir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		bridgeConfigDir := filepath.Join(tmpdir, "bridge-config")
		err = os.Mkdir(bridgeConfigDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		svc := fmt.Sprintf(string(svctemplate), fmt.Sprintf("\"%s\", \"-gloo-address\", \"localhost\", \"--gloo-port\", \"8081\", \"--conf-dir\",\"%s\", \"--envoy-path\",\"%s\"", pathToGlooBridge, bridgeConfigDir, envoypath))

		consulConfigDir = filepath.Join(tmpdir, "consul-config")
		err = os.Mkdir(consulConfigDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(consulConfigDir, "service.json"), []byte(svc), 0644)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		gexec.KillAndWait()
		consulSession = nil

		if tmpdir != "" {
			os.RemoveAll(tmpdir)
		}
	})

	runConsul := func() {
		consul := exec.Command("consul", "agent", "-dev", "--config-dir="+consulConfigDir)
		session, err := gexec.Start(consul, GinkgoWriter, GinkgoWriter)
		consulSession = session

		Expect(err).NotTo(HaveOccurred())
	}

	var networkListener net.Listener

	runFakeXds := func(bindadr string, port uint) {
		var filterChains []envoylistener.FilterChain = []envoylistener.FilterChain{{
			Filters: []envoylistener.Filter{{
				Name: "envoy.echo",
			}},
		}}

		l := &envoyapi.Listener{
			Name: "test",
			Address: envoycore.Address{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Protocol: envoycore.TCP,
						Address:  bindadr,
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: uint32(port),
						},
						Ipv4Compat: true,
					},
				},
			},
			FilterChains: filterChains,
		}
		var listenersProto []envoycache.Resource
		listenersProto = append(listenersProto, l)

		snap := envoycache.NewSnapshot("v1", nil, nil, nil, listenersProto)

		envoyCache := envoycache.NewSnapshotCache(true, fakehasher{}, fakelogger{})
		envoyCache.SetSnapshot("test", snap)

		grpcServer := grpc.NewServer()

		xdsServer := xds.NewServer(envoyCache, nil)
		envoyv2.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
		v2.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
		v2.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
		v2.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
		v2.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)
		lis, err := net.Listen("tcp", ":8081")
		Expect(err).NotTo(HaveOccurred())
		networkListener = lis
		go grpcServer.Serve(networkListener)
	}

	AfterEach(func() {
		if networkListener != nil {
			networkListener.Close()
			networkListener = nil
		}
	})

	It("should start envoy", func() {
		runConsul()
		time.Sleep(1 * time.Second)
		Expect(consulSession).ShouldNot(gexec.Exit())
		Eventually(consulSession.Out).Should(gbytes.Say("agent/proxy: starting proxy:"))

		// check that a port was opened where consul says it should have been opened (get the port from consul connect and check that it is open)
		resp, err := http.Get("http://127.0.0.1:8500/v1/agent/connect/proxy/web-proxy")
		Expect(err).NotTo(HaveOccurred())
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		Expect(err).NotTo(HaveOccurred())

		var cfg api.ProxyInfo
		json.Unmarshal(body, &cfg)

		runFakeXds(cfg.Config.BindAddress, cfg.Config.BindPort)
		time.Sleep(10 * time.Second)
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Config.BindAddress, cfg.Config.BindPort))
		Expect(err).NotTo(HaveOccurred())

		// We are connected! good enough!
		conn.Close()
	})
})

type fakehasher struct{}

func (fakehasher) ID(*envoycore.Node) string {
	return "test"
}

type fakelogger struct{}

func (fakelogger) Errorf(format string, args ...interface{}) {

}
func (fakelogger) Infof(format string, args ...interface{}) {

}
