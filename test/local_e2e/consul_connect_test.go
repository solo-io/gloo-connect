package local_e2e

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
	"github.com/solo-io/gloo/pkg/log"
)

type ConsulService struct {
	Service Service `json:"service"`
}

type Service struct {
	Name    string  `json:"name"`
	Port    int     `json:"port"`
	Connect Connect `json:"connect"`
}

type Connect struct {
	Proxy Proxy `json:"proxy"`
}

type Proxy struct {
	ExecMode string   `json:"exec_mode"`
	Command  []string `json:"command"`
	Config   Config   `json:"config"`
}

type Config struct {
	Upstreams []Upstream `json:"upstreams"`
}

type Upstream struct {
	DestinationName string `json:"destination_name"`
	LocalBindPort   int    `json:"local_bind_port"`
}

type ProxyInfo struct {
	ProxyServiceID    string
	TargetServiceID   string
	TargetServiceName string
	ContentHash       string
	ExecMode          string
	Command           []string
	Config            ProxyConfig
}

type ProxyConfig struct {
	BindAddress         string     `json:"bind_address"`
	BindPort            uint       `json:"bind_port"`
	LocalServiceAddress string     `json:"local_service_address"`
	Upstreams           []Upstream `json:"upstreams"`
}

var _ = Describe("ConsulConnect", func() {
	var tmpdir string
	var consulConfigDir string
	var bridgeConfigDir string
	var pathToGlooBridge string
	var envoypath string
	var consulSession *gexec.Session
	xdsPort := 7071

	var waitForInit time.Duration = 5 * time.Second

	BeforeSuite(func() {
		var err error
		pathToGlooBridge, err = gexec.Build("github.com/solo-io/gloo-connect/cmd")
		Expect(err).ShouldNot(HaveOccurred())

		envoypath = os.Getenv("ENVOY_PATH")
		if envoypath == "" {
			envoypath, _ = exec.LookPath("envoy")
		}
		if envoypath == "" {
			log.Warnf("running envoy from /usr/local/bin/envoy. to override, set ENVOY_PATH")
			envoypath = "/usr/local/bin/envoy"
		}

	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	writeService := func(uds bool) {
		// generate the template
		args := []string{
			pathToGlooBridge,
			"bridge",
			"--gloo-port",
			fmt.Sprintf("%v", xdsPort),
			"--conf-dir",
			bridgeConfigDir,
			"--envoy-path",
			envoypath,
		}
		if uds {
			args = append(args, "--gloo-uds=true")
		}

		if os.Getenv("USE_DLV") == "1" {
			dlv, err := exec.LookPath("dlv")
			if err == nil {
				dlvargs := []string{dlv, "exec", "--headless", "--listen", "localhost:2345", "--"}
				args = append(dlvargs, args...)
			}
			waitForInit = time.Hour
		}

		svc := ConsulService{
			Service: Service{
				Name: "web",
				Port: 9090,
				Connect: Connect{
					Proxy: Proxy{
						ExecMode: "daemon",
						Command:  args,
						Config: Config{
							Upstreams: []Upstream{
								{
									DestinationName: "consul",
									LocalBindPort:   1234,
								},
							},
						},
					},
				},
			},
		}

		data, err := json.Marshal(svc)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(consulConfigDir, "service.json"), data, 0644)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		tmpdir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		bridgeConfigDir = filepath.Join(tmpdir, "glooBridge-config")
		err = os.Mkdir(bridgeConfigDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		consulConfigDir = filepath.Join(tmpdir, "consul-config")
		err = os.Mkdir(consulConfigDir, 0755)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		gexec.TerminateAndWait("5s")
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

	It("should start envoy with gloo tcp", func() {
		writeService(false)
		runConsul()
		time.Sleep(1 * time.Second)
		Expect(consulSession).ShouldNot(gexec.Exit())
		Eventually(consulSession.Out, "5s").Should(gbytes.Say("agent/proxy: starting proxy:"))

		// check that a port was opened where consul says it should have been opened (get the port from consul connect and check that it is open)
		resp, err := http.Get("http://127.0.0.1:8500/v1/agent/connect/proxy/web-proxy")
		Expect(err).NotTo(HaveOccurred())
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		Expect(err).NotTo(HaveOccurred())

		var cfg ProxyInfo
		json.Unmarshal(body, &cfg)

		//runFakeXds(cfg.Config.BindAddress, cfg.Config.BindPort)

		time.Sleep(waitForInit)

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Config.BindAddress, cfg.Config.BindPort))
		Expect(err).NotTo(HaveOccurred())

		// We are connected! good enough!
		conn.Close()
	})

	It("should start envoy with gloo uds", func() {
		writeService(true)
		runConsul()
		time.Sleep(1 * time.Second)
		Expect(consulSession).ShouldNot(gexec.Exit())
		Eventually(consulSession.Out, "5s").Should(gbytes.Say("agent/proxy: starting proxy:"))

		// check that a port was opened where consul says it should have been opened (get the port from consul connect and check that it is open)
		resp, err := http.Get("http://127.0.0.1:8500/v1/agent/connect/proxy/web-proxy")
		Expect(err).NotTo(HaveOccurred())
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		Expect(err).NotTo(HaveOccurred())

		var cfg ProxyInfo
		json.Unmarshal(body, &cfg)

		//runFakeXds(cfg.Config.BindAddress, cfg.Config.BindPort)

		time.Sleep(waitForInit)

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Config.BindAddress, cfg.Config.BindPort))
		Expect(err).NotTo(HaveOccurred())

		// We are connected! good enough!
		conn.Close()
	})
})
