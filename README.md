

<h1 align="center">
    <img src="docs/GlooConnect.png" alt="GlooConnect" width="428" height="242">
  <br>
  The Consul-Native Service Mesh
</h1>

### What is Gloo Connect?

Gloo Connect is built on top of [Consul](https://github.com/hashicorp/consul) and [Gloo](https://github.com/solo-io/gloo) to enable real time route authorization and response configuration management. Gloo Connect leverages Consul's Level 3/4 TCP authentication and Gloo's Level 7 function routing to provide a complete secure, observable, and configurable service delivery environment.

<BR>
<p align="center">
    <img src="docs/figures/overview.png" alt="GlooConnect_overview" width="800" height="478">
</p>

<BR>
    
## Gloo Connect Features
### Consul Connect: Layer 3/4
* **Security**: AuthN, AuthZ, intents, policy, mTLS
* **Networking**: TCP routing 
### Gloo Connect: Layer 7
* **Security**: RBAC
* **Observability**: Analytics, Monitoring, Debugging, Logging
* **Traffic control**: Rate limit, Transformation, Compression, Caching, Retries, Canary deployment
* **Networking**: HTTP routing, serverless
<BR>
    

## Architecture Overview
<BR>
<p align="center">
    <img src="docs/figures/consul_gloo_integration.png" alt="GlooConnect_architecture" width="800" height="422">
</p>
<BR>

## Documentation
* [Gloo Connect Documentation](https://connect.solo.io)


Blogs & Demos
-----
* [Announcement Blog](https://medium.com/solo-io/)

Community
-----
Join us on our slack channel: [https://slack.solo.io/](https://slack.solo.io/)

---

### Thanks

**Gloo Connect** would not be possible without the valuable open-source work of projects in the community. We would like to extend 
a special thank-you to [Envoy](https://www.envoyproxy.io) and [Consul](https://github.com/hashicorp/consul).
