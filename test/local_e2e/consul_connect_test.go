package local_e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

var _ = Describe("ConsulConnect", func() {
	var tmpdir string
	var consulConfigDir string
	var bridgeConfigDir string
	var pathToGlooBridge string
	var envoypath string
	var consulSession *gexec.Session
	xdsPort := 7071

	var waitForInit time.Duration = 10 * time.Second

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

	serviceWritten := false
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
		serviceWritten = true
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
		gexec.TerminateAndWait("10s")
		consulSession = nil

		if tmpdir != "" {
			os.RemoveAll(tmpdir)
		}
	})

	runConsul := func() {
		if !serviceWritten {
			writeService(false)
		}
		consulSession = RunConsul(consulConfigDir)
	}
	waitForProxy := func() {
		// time.Sleep(1 * time.Second)
		// Expect(consulSession).ShouldNot(gexec.Exit())
		Eventually(consulSession.Out, "10s").Should(gbytes.Say("agent/proxy: starting proxy:"))
	}
	runConsulAndWait := func() {
		runConsul()
		waitForProxy()
	}

	It("should start envoy with gloo tcp", func() {
		runConsulAndWait()
		// check that a port was opened where consul says it should have been opened (get the port from consul connect and check that it is open)
		cfg := GetProxyInfo()
		Eventually(func() error { return TestPortOpen(cfg.Config.BindAddress, cfg.Config.BindPort) }, waitForInit, "1s").Should(BeNil())
	})

	It("should start envoy with gloo uds", func() {
		writeService(true)
		runConsulAndWait()
		cfg := GetProxyInfo()
		Eventually(func() error { return TestPortOpen(cfg.Config.BindAddress, cfg.Config.BindPort) }, waitForInit, "1s").Should(BeNil())
	})

})
