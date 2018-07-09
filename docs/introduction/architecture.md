# Architecture

The architecture of Gloo Connect can be understood as follows:

![Architecture](../../figures/consul_gloo_integration.png "Gloo Connect Architecture")

A new service with connect settings is added to [Consul](https://www.consul.io/docs/index.html) using Gloo Connect as a managed proxy. 

Consul then initiates the Gloo-Consul Bridge process, where Gloo-Consul Bridge retrieves certificates and configuration information from Consul. Certificates are then written to disk to make them available to [Envoy](https://www.envoyproxy.io/docs/envoy/latest/).

In the near future, certificates will be delivered directly to Envoy over SDS and not written to disk. This process configures Envoy with the control plane settings and starts it. 

Gloo Connect monitors Consul Connect and updates the control plane whenever changes are detected and Envoy is hot-restarted. Envoy receives configuration from the Gloo Connect control plane and serves mesh data!

The data flows as follows: 

1. A request arrives from the mesh to the local Envoy.
2. Gloo Connect filter checks if the connection is authorized with the local agent.
3. If the request is authorized by the agent it is forwarded to the service.
