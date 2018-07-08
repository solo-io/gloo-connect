package local_e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/consul/api"

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

	var testContext context.Context
	var cancel context.CancelFunc

	xdsPort := 7071
	svcPort := 9090

	var waitForInit time.Duration = 10 * time.Second

	BeforeSuite(func() {
		var err error
		pathToGlooBridge, err = gexec.Build("github.com/solo-io/gloo-connect/cmd", "-gcflags", "all=-N -l")
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

	getWebService := func(args []string) Service {
		return Service{
			Name: "web",
			Port: svcPort,
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
		}
	}

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
			Service: getWebService(args),
		}

		data, err := json.Marshal(svc)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(consulConfigDir, "service.json"), data, 0644)
		Expect(err).NotTo(HaveOccurred())
		serviceWritten = true
	}

	writeServices2 := func(two bool) {
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
			"--gloo-uds=true",
		}
		var svcs []Service

		if two == false && os.Getenv("USE_DLV") == "1" {
			dlv, err := exec.LookPath("dlv")
			Expect(err).ToNot(HaveOccurred())
			dlvargs := []string{dlv, "exec", "--headless", "--listen", "localhost:2345", "--"}
			dlvargs = append(dlvargs, args...)
			waitForInit = time.Hour
			svcs = []Service{getWebService(dlvargs)}
		} else {
			svcs = []Service{getWebService(args)}
		}

		if two {
			if os.Getenv("USE_DLV") == "1" {
				dlv, err := exec.LookPath("dlv")
				if err == nil {
					dlvargs := []string{dlv, "exec", "--headless", "--listen", "localhost:2345", "--"}
					args = append(dlvargs, args...)
				}
				waitForInit = time.Hour
			}

			svcs = append(svcs, Service{
				Name: "test",
				Port: svcPort + 1,
				Connect: Connect{
					Proxy: Proxy{
						ExecMode: "daemon",
						Command:  args,
						Config: Config{
							Upstreams: []Upstream{
								{
									DestinationName: "web",
									LocalBindPort:   1334,
								},
							},
						},
					},
				},
			})
		}

		svc := ConsulServices{
			Services: svcs,
		}

		data, err := json.Marshal(svc)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(consulConfigDir, "service.json"), data, 0644)
		Expect(err).NotTo(HaveOccurred())
		serviceWritten = true
	}

	writeServices := func() {
		writeServices2(true)
	}
	writeServiceInArray := func() {
		writeServices2(false)
	}

	BeforeEach(func() {
		testContext, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
		testContext = nil
		cancel = nil
	})
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

	It("should start envoy with gloo tcp registration as services", func() {
		writeServiceInArray()
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

	It("should start envoy with gloo http", func() {
		// pretend we are the webservice, fail the first request, and wait for the second one.
		// start the web service before so that consul can detect as healthy
		i := 0
		handlerfunc := func(rw http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/test" {
				return
			}
			if i == 0 {
				rw.WriteHeader(http.StatusInternalServerError)
			}
			i++
		}

		handler := http.HandlerFunc(handlerfunc)
		go func() {
			h := &http.Server{Handler: handler, Addr: fmt.Sprintf(":%d", svcPort)}
			go func() {
				if err := h.ListenAndServe(); err != nil {
					if err != http.ErrServerClosed {
						panic(err)
					}
				}
			}()

			<-testContext.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			h.Shutdown(ctx)
			cancel()
		}()
		writeServices()
		runConsul()
		// configHTTPCmd := exec.Command(pathToGlooBridge, "http")
		configHTTPCmd := exec.Command(pathToGlooBridge, "set", "service", "web", "--retries=10", "--http")
		configHTTPCmd.Stderr = GinkgoWriter
		configHTTPCmd.Stdout = GinkgoWriter
		err := configHTTPCmd.Run()
		Expect(err).NotTo(HaveOccurred())
		waitForProxy()

		cfg := GetProxyInfo()
		// time.Sleep(time.Hour)

		Eventually(func() error { return TestPortOpen(cfg.Config.BindAddress, cfg.Config.BindPort) }, waitForInit, "1s").Should(BeNil())

		client, err := api.NewClient(api.DefaultConfig())
		Expect(err).NotTo(HaveOccurred())

		// let everything settle. give time for envoy to start and for consul time to do health checks.
		Eventually(
			func() int { se, _, _ := client.Health().Connect("web", "", true, nil); return len(se) },
			"20s",
			"1s",
		).ShouldNot(BeZero())
		Eventually(
			func() int { se, _, _ := client.Health().Connect("test", "", true, nil); return len(se) },
			"20s",
			"1s",
		).ShouldNot(BeZero())

		// connect to the web service from the test service
		resp, err := http.Get("http://localhost:1334/test")

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))

		// the service will fail the first request, and envoy should try again
		// hence a count of 2 test requests
		Expect(i).To(Equal(2))
	})
})
