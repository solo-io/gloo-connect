package util

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

func PrintConsulServices() {
	s, _ := ListConsulServices(api.DefaultConfig())
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
