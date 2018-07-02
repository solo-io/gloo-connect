package gloo_test

import (
	"encoding/json"

	"github.com/hashicorp/consul/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/solo-io/gloo-connect/pkg/gloo"
)

const ProxyJson = `{"ProxyServiceID":"web-proxy","TargetServiceID":"web","TargetServiceName":"web","ContentHash":"79e3379ba6636791","ExecMode":"daemon","Command":["/home/yuval/bin/gloo-connect","--gloo-address","localhost","--gloo-port","8081","--conf-dir","/home/yuval/go/src/github.com/solo-io/gloo-connect/getting-started/t/gloo-connect-config","--envoy-path","/home/yuval/bin/envoy","--storage.type","file","--secrets.type","file","--files.type","file","--file.config.dir","/home/yuval/go/src/github.com/solo-io/gloo-connect/getting-started/t/gloo/_gloo_config","--file.files.dir","/home/yuval/go/src/github.com/solo-io/gloo-connect/getting-started/t/gloo/_gloo_config/files","--file.secret.dir","/home/yuval/go/src/github.com/solo-io/gloo-connect/getting-started/t/gloo/_gloo_config/secrets"],"Config":{"bind_address":"127.0.0.1","bind_port":20143,"local_service_address":"127.0.0.1:8080","yuval":"yuval"}}`

var _ = Describe("Configwriter", func() {

	It("should deserialize correctly", func() {
		var pcfg api.ConnectProxyConfig
		err := json.Unmarshal([]byte(ProxyJson), &pcfg)
		Expect(err).NotTo(HaveOccurred())

		cfg, err := GetProxyConfig(&pcfg)

		Expect(err).NotTo(HaveOccurred())

		Expect(cfg.BindPort).NotTo(BeZero())
		Expect(cfg.BindAddress).NotTo(BeEmpty())
	})

})
