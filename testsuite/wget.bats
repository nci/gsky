#!/bin/bash

function create_config_file() {
    cat >$1 <<EOF
	{ "service_config": {
	    "ows_hostname": "gsky.example.com",
	    "mas_address": "MAS_IP:80",
	    "worker_nodes": ["127.0.0.1:6000"]
          }
	}
EOF
}

function setup() {
    create_config_file config.json
    WGET="wget --quiet -O- http://localhost:8080"
    $GOPATH/bin/gsky &
    pid=$!
    disown -r
    sleep 2  # give the server time to start
}

function teardown() {
    kill $pid
    rm config.json
}

@test "basic capabilities test" {
    run $WGET
    [ "$status" -eq 0 ]
    [[ "$output" =~ a\ distributed\ geospatial\ data\ server ]]
}
