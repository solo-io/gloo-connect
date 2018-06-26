# Architecture

The Architecture of QLoo can be understood as follows:


QLoo users interact via the [Storage Layer API](https://github.com/solo-io/qloo/tree/master/pkg/storage).

written by the User (usually via `qlooctl`, the QLoo CLI) and polled by QLoo.

When QLoo detects an update to an API Object, it re-syncs its state to match
the user specified configuration.

QLoo is composed of two components: a GraphQL service and an Envoy Proxy functioning as a sidecar. Rather than manually configuring
its own sidecar, QLoo directs Envoy to connect to Gloo as its [control plane](https://github.com/envoyproxy/data-plane-api/blob/master/XDS_PROTOCOL.md), 
allowing QLoo to leverage [Gloo's large set of HTTP routing features](https://gloo.solo.io/#features).

to handle service discovery, [Gloo plugin configuration](https://gloo.solo.io/plugins/aws/), and configuration of 
[Envoy HTTP Filters](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http_filters.html)

Once Gloo has applied the desired configuration to Envoy, QLoo begins listening for incoming GraphQL requests, serving queries 
