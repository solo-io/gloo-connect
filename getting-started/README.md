# Gloo Consul integration

# Configuration

Binaries that we will use:

- A consul binary in our path
- A envoy binary with gloo plugins
- A gloo consul bridge binary

Additionally, you will need a service to connect to. we have packaged an example go service 
in service.go file.

# Prepare environment

Commands should be run in a terminal, and cd to the folder containing this file. 
Before running each command we will source `prepare-env.sh` to create folders and environment variables.

## Configure consul

Register the consul bridge:

```bash
#  Create folders and environment variables
. prepare-env.sh

cat > $CONSUL_CONFIG_HOME/connect.json <<EOF
{
    "connect" : {
    "enabled" : true,
    "proxy_defaults" : {
        "exec_mode" : "daemon",
        "daemon_command" : ["${GLOO_BRIDGE_PATH}", "--gloo-address", "localhost", "--gloo-port", "8081", "--conf-dir","${GLOO_CONSUL_BRIDGE_HOME}", "--envoy-path","${ENVOY_PATH}",
        "--storage.type","file",
        "--secrets.type","file",
        "--files.type","file",
        "--file.config.dir","${GLOO_CONFIG_HOME}/_gloo_config",
        "--file.files.dir","${GLOO_CONFIG_HOME}/_gloo_config/files",
        "--file.secret.dir","${GLOO_CONFIG_HOME}/_gloo_config/secrets"
        ]
        }
    }
}
EOF
```

## Start and register a microservice

Start your test service:


```
go run service.go&
``` 

...and reigster it with consul:
```
cat > $CONSUL_CONFIG_HOME/service.json <<EOF
{
    "service": {
        "name": "web",
        "port": 8080,
        "connect": {
            "proxy": {}
        }
    }
}
EOF
```

Start consul:

```
consul agent -dev --config-dir=$CONSUL_CONFIG_HOME
```

# What's happening
At this point, consul will start the the gloo bridge as a managed proxy, that in turn will start and configure gloo and envoy to be part of the mesh.

## Test!

Use consul to create a proxy (as specified in the debugging section in the consul docs),
and use curl to test!:
```
consul connect proxy \
  -service operator-test \
  -upstream web:8181
curl http://localhost:8181
```


