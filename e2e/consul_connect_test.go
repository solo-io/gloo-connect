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

		svc := fmt.Sprintf(string(svctemplate), fmt.Sprintf("\"%s\", \"-gloo-address\", \"localhost\", \"--gloo-port\", \"8080\", \"--conf-dir\",\"%s\", \"--envoy-path\",\"%s\"", pathToGlooBridge, bridgeConfigDir, envoypath))

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

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Config.BindAddress, cfg.Config.BindPort))
		Expect(err).NotTo(HaveOccurred())

		// We are connected! good enough!
		conn.Close()
	})

})
