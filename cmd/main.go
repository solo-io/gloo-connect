package main

import (
	"flag"

	"github.com/solo-io/consul-gloo-bridge/pkg/runner"
)

func main() {
	var rc runner.RunConfig

	flag.StringVar(&rc.GlooAddress, "gloo-address", "", "address for gloo ADS server")
	flag.UintVar(&rc.GlooPort, "gloo-port", 0, "port for gloo ADS server")
	flag.StringVar(&rc.ConfigDir, "conf-dir", "", "config dir to hold envoy config file")
	flag.Parse()

	runner.Run(rc)
}
