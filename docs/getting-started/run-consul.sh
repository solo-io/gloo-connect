ENVOY_PATH=$(which envoy)
GLOO_CONNECT_PATH=$(which gloo-connect)

if [ -f $PWD/envoy ]; then
ENVOY_PATH=$PWD/envoy
fi

if [ -f $PWD/gloo-connect ]; then
GLOO_CONNECT_PATH=$PWD/gloo-connect
fi


if [ -z "$ENVOY_PATH" ]; then
echo "Cant find envoy - please make sure envoy is in your path or in the current directory"
exit 1
fi

if [ -z "$GLOO_CONNECT_PATH" ]; then
echo "Cant find gloo-connect - please make sure gloo-connect is in your path or in the current directory"
exit 1
fi


CONSUL_CONFIG_HOME=${PWD}/run-data/consul-config
mkdir -p $CONSUL_CONFIG_HOME

GLOO_CONSUL_BRIDGE_HOME=${PWD}/run-data/gloo-connect-config
mkdir -p $GLOO_CONSUL_BRIDGE_HOME

cat > $CONSUL_CONFIG_HOME/connect.json <<EOF
{
    "connect" : {
    "enabled" : true,
    "proxy_defaults" : {
        "exec_mode" : "daemon",
        "daemon_command" : [
        "${GLOO_CONNECT_PATH}",
        "bridge",
        "--conf-dir","${GLOO_CONSUL_BRIDGE_HOME}",
        "--gloo-uds",
        "--envoy-path","${ENVOY_PATH}"
        ]
        }
    }
}
EOF

cat > $CONSUL_CONFIG_HOME/service.json <<EOF
{
    "services": [
        {
            "name": "microsvc1",
            "port": 8080,
            "connect": {
                "proxy": {}
            }
        },
        {
            "name": "test",
            "port": 9091,
            "connect": {
                "proxy": {
                    "config": {
                        "upstreams": [
                            {
                                "destination_name": "microsvc1",
                                "local_bind_port": 1234
                            }
                        ]
                    }
                }
            }
        }
    ]
}
EOF

cd run-data
consul agent -dev --config-dir=$CONSUL_CONFIG_HOME
