package glooclient

import (
	"fmt"

	"github.com/solo-io/gloo/pkg/plugins/consul"

	"time"

	"github.com/gogo/protobuf/types"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/solo-io/gloo/pkg/storage"

	"github.com/solo-io/gloo/pkg/coreplugins/route-extensions"
)

type GlooClient struct {
	Store storage.Interface
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

const (
	allOrigins = "all-origins"
)

func (c *GlooClient) ConfigureService(serviceType string, retries uint32, timeout time.Duration) error {
	res := extensions.RouteExtensionSpec{
		MaxRetries: retries,
	}
	if timeout > 0 {
		res.Timeout = timeout
	}
	return c.EnableBasicHttp("", serviceType, extensions.EncodeRouteExtensionSpec(res))
}

func (c *GlooClient) Demo() error {
	return c.EnableBasicHttp("", "web", extensions.EncodeRouteExtensionSpec(extensions.RouteExtensionSpec{
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
	vService, err := c.Store.V1().VirtualServices().Get(name)
	if err != nil {
		vService, err = c.Store.V1().VirtualServices().Create(&v1.VirtualService{
			Name:               name,
			Domains:            []string{"*"},
			DisableForGateways: true,
		})
		if err != nil {
			return err
		}
	}
	// TODO: merge routes
	vService.Routes = nil
	vService.Routes = append(vService.Routes, &v1.Route{
		Extensions: route.Config,
		Matcher:    &v1.Route_RequestMatcher{RequestMatcher: route.Matcher},
		SingleDestination: &v1.Destination{
			DestinationType: &v1.Destination_Upstream{
				Upstream: &v1.UpstreamDestination{
					Name: consul.UpstreamNameForConnectService(destination),
				},
			},
		},
	})
	vService, err = c.Store.V1().VirtualServices().Update(vService)
	if err != nil {
		return err
	}

	// TODO(yuval-k): refactor these keys to a shared package with https://github.com/solo-io/gloo-connect/pull/13/files#diff-dd009a95782c9f59f4baeadcd504edd6R181
	selector := map[string]string{
		"destination": destination,
	}
	if origin != allOrigins {
		selector["service"] = origin
	}

	attribute, err := c.Store.V1().Attributes().Get(name)
	if err != nil {
		attribute, err = c.Store.V1().Attributes().Create(&v1.Attribute{
			Name: name,
			AttributeType: &v1.Attribute_ListenerAttribute{
				ListenerAttribute: &v1.ListenerAttribute{}},
		})
		if err != nil {
			return err
		}
	}

	attribute.AttributeType = &v1.Attribute_ListenerAttribute{
		ListenerAttribute: &v1.ListenerAttribute{
			Selector:        selector,
			VirtualServices: []string{name},
		},
	}
	_, err = c.Store.V1().Attributes().Update(attribute)
	return err
}

func virtualServiceName(origin, destination string) string {
	return fmt.Sprintf("%v-to-%v-routes", origin, destination)
}
