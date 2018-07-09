package cmd

import (
	"github.com/gogo/protobuf/types"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"github.com/solo-io/gloo/pkg/storage"
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
