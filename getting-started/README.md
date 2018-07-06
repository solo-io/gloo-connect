# Gloo Consul integration

# Background

Binaries that we will use:

- A Consul binary in our path
- A envoy binary with gloo plugins (specifically the envoy-consul-connect plugin that enables support for Consul Connect intentions based auth)
- A gloo-connect binary

You can find all three binaries in the release bundle.

## Demo web  µ-service

In this guide we will run a demo µ-service to simulate a microservice with an HTTP interface. 
The source of the service is in service.go.
This simulated web µ-service returns the contents of the '/usr/share/doc' folder.
The service fails every other request, and we will use it to demonstrate L7 (http) features.

# Prepare environment

Commands should be run in a terminal. cd to the folder containing this file. 
The envoy and gloo-connect binarie should be either in this folder, or in your path.

## Start and register a microservice

Start your demo µ-service to run in the background:

```
go run service.go&
``` 

## Run consul

Configure and run Consul:

```bash
#  This script create folders, environment variables, consul configuration and starts consul.
./run-consul.sh
```


# Enter the mesh
At this point, Consul will start gloo-connect as a managed proxy, that in turn will start and configure envoy as a part of the mesh.

## Test!

We configured Consul with an additional test service. The demo µ-service is configured as an upstream for the test service, using port 1234. This any connection through port 1234 is a connection made to the web service as the test service. So to test our mesh, all we need to do is this:
```
curl http://localhost:1234
```
Remember - As the microservice fails every other request, you will see a different result on each invocation of curl. Great!

## L7 in Action

As you can see, when you curl the service multiple times it sometimes fails ☹. To overcome this, we can use gloo-connect layer 7 features and add a re-try:
```
gloo-connect http set service web --retries=3
```

This tells configures gloo and envoy, and instructs them to automatically re-try requests sent to the service up to 3 times.

Now try:
```
curl http://localhost:1234
```

And you will see it work consistently!

# Cleanup:

To stop Consul, just hit CNTRL+C and let it terminate gracefully (which will terminate the other processes as well).

Some configuration files and logs were created in run-data folder. to clean up just remove the folder:
```
rm -rf ./run-data
```