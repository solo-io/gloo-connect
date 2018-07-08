package util

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/solo-io/gloo/pkg/bootstrap"
)

func PrintConsulServices(o *bootstrap.ConsulOptions) {
	//o := bootstrap.ConsulOptions{}
	s, _ := ListConsulServices(o.ToConsulConfig())
	fmt.Println("consul services: ", s)
}

func ListConsulServices(cfg *api.Config) ([]string, error) {
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	services, _, err := client.Catalog().Services(nil)
	if err != nil {
		return nil, err
	}
	var servicenames []string
	for svcname := range services {
		// filter out proxy services
		if !strings.HasSuffix(svcname, "-proxy") {
			servicenames = append(servicenames, svcname)
		}
	}
	return servicenames, nil
}
