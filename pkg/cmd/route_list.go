package cmd

import (
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/gogo/protobuf/types"
	"github.com/solo-io/gloo/pkg/storage"
	"fmt"
	"github.com/pkg/errors"
	"github.com/solo-io/gloo/pkg/coreplugins/route-extensions"
	"time"
	"github.com/solo-io/gloo/pkg/plugins/connect"
)

const (
	allOrigins = "all-origins"
)

type GlooClient struct {
	gloo storage.Interface
}

type RouteList struct {
	// name of the origin service where the routes will be applied
	// leave empty to apply to all origins
	OriginServiceName string `json:"origin_service_name"`
	// name of the upstream service for all the routes
	// leave empty to apply to all destinations
	DestinationServiceName string `json:"destination_service_name"`
}

type Route struct {
	Matcher *v1.RequestMatcher `json:"matcher"`
	Config  *types.Struct      `json:"config"`
	// Destination is implicit
}

// TODO: remove lol
func (c *GlooClient) demo() {
	c.EnableBasicHttp("web", "db", extensions.EncodeRouteExtensionSpec(extensions.RouteExtensionSpec{
		MaxRetries: 10,
		Timeout:    time.Minute,
	}))
}

func (c *GlooClient) EnableBasicHttp(origin, destination string, config *types.Struct) error {
	return c.AddRoute(origin, destination, Route{
		Matcher: &v1.RequestMatcher{
			Path: &v1.RequestMatcher_PathPrefix{
				PathPrefix: "/",
			},
		},
		Config: config,
	})
}

// Currently routes are only supported on outbound listeners
// TODO(ilackarms): modify here and connect/plugin.go to support both ways
func (c *GlooClient) AddRoute(origin, destination string, route Route) error {
	if origin == "" {
		origin = allOrigins
	}
	name := virtualServiceName(origin, destination)
	vService, err := c.gloo.V1().VirtualServices().Get(name)
	if err != nil {
		vService, err = c.gloo.V1().VirtualServices().Create(&v1.VirtualService{
			Name:               name,
			Domains:            []string{"*"},
			DisableForGateways: true,
		})
		if err != nil {
			return err
		}
	}
	vService.Routes = append(vService.Routes, &v1.Route{
		Matcher: &v1.Route_RequestMatcher{RequestMatcher: route.Matcher},
		SingleDestination: &v1.Destination{
			DestinationType: &v1.Destination_Upstream{
				Upstream: &v1.UpstreamDestination{
					// TODO(yuval-k): make sure destination name matches the upstream name as known to gloo
					// need to make sure upstream is added without tags (see gloo/pkg/plugins/consul/plugin.go)
					Name: destination,
				},
			},
		},
	})
	// TODO(yuval-k): refactor these keys to a shared package with https://github.com/solo-io/gloo-connect/pull/13/files#diff-dd009a95782c9f59f4baeadcd504edd6R181
	selector := map[string]string{
		"destination": destination,
	}
	if origin != allOrigins {
		selector["service"] = origin
	}
	attribute := &v1.Attribute{
		AttributeType: &v1.Attribute_ListenerAttribute{
			ListenerAttribute: &v1.ListenerAttribute{
				Selector: selector,
				VirtualServices: []string{name},
			},
		},
	}
	c.gloo.V1().Attributes().Create()
}

func virtualServiceName(origin, destination string) string {
	return fmt.Sprintf("%v-to-%v-routes", origin, destination)
}
