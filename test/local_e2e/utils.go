package local_e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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

func GetProxyInfo() ProxyInfo {

	// check that a port was opened where consul says it should have been opened (get the port from consul connect and check that it is open)
	resp, err := http.Get("http://127.0.0.1:8500/v1/agent/connect/proxy/web-proxy")
	Expect(err).NotTo(HaveOccurred())
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	Expect(err).NotTo(HaveOccurred())

	var cfg ProxyInfo
	json.Unmarshal(body, &cfg)
	return cfg
}

func TestPortOpen(address string, port uint) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), time.Second/2)
	if err == nil {
		// We are connected! good enough!
		conn.Close()
		return nil
	}
	return err
}

func RunConsul(consulConfigDir string) *gexec.Session {
	consul := exec.Command("consul", "agent", "-dev", "--config-dir="+consulConfigDir)
	session, err := gexec.Start(consul, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	// wait for consul to start
	time.Sleep(time.Second)

	return session
}
