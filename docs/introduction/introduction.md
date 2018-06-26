# Introduction

### What is Gloo Connect?

Gloo Connect is built on top of [Consul](https://github.com/hashicorp/consul) and [Gloo](https://github.com/solo-io/gloo) to enable real time route authorization and response configuration management. Gloo Connect leverages Consul's Level 3/4 TCP authentication and Gloo's Level 7 function routing to provide a complete secure, observable, and configurable service delivery environment.
<BR>

Gloo Connect links a Consul Connect filter with the Envoy binary.
The filter performs TLS client authentication against the Authorize endpoint via REST API.

The Authorize endpoint tests whether a connection attempt is authorized between two services.
Consul’s implementation of this API uses locally cached data and doesn't require any request forwarding to a server. Therefore, the response typically occurs in microseconds, to impose minimal overhead on the connection attempt.

The filter provides the presented client certificate information to the Authorize endpoint in order to determine whether the connection should be allowed or not.


