# Gloo Consul integration

# Configuration

You will need:
- A consul binary
- A envoy binary with gloo plugins
- A gloo consul bridge binary
- A localgloo binary
- A service that you want to connect to

Start localgloo

```bash
export GLOO_CONFIG_HOME=${PWD}/gloo
mkdir $GLOO_CONFIG_HOME
localgloo \
  --storage.type file \
  --secrets.type file \
  --files.type file \
  --file.config.dir ${GLOO_CONFIG_HOME}/_gloo_config \
  --file.files.dir ${GLOO_CONFIG_HOME}/_gloo_config/files \
  --file.secret.dir ${GLOO_CONFIG_HOME}/_gloo_config/secrets
```

Register the consul bridge:

```
export CONSUL_CONFIG_HOME=${PWD}/consul-config
mkdir $CONSUL_CONFIG_HOME
export GLOO_CONSUL_BRIDGE=${PWD}/gloo-consul-bridge-config
mkdir $GLOO_CONSUL_BRIDGE

GLOO_BRIDGE_PATH=gloo-consul-bridge
ENVOY_PATH=/home/yuval/bin/envoy

cat > $CONSUL_CONFIG_HOME/connect.json <<EOF
{
    "connect" : {
    "enabled" : true,
    "proxy_defaults" : {
        "exec_mode" : "daemon",
        "daemon_command" : ["${GLOO_BRIDGE_PATH}", "--gloo-address", "localhost", "--gloo-port", "8081", "--conf-dir","${GLOO_CONSUL_BRIDGE}", "--envoy-path","${ENVOY_PATH}",
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

Start your test service:

```
go run service.go&
```

create a consul configuration for it:
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

Connect and test!

```
consul connect proxy \
  -service operator-test \
  -upstream web-proxy:8181
curl http://localhost:8181
```
