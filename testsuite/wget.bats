#!/bin/bash

function create_config_file() {
    cat >$1 <<EOF
	{ "service_config": {
	    "ows_hostname": "gsky.example.com",
	    "mas_address": "MAS_IP:80",
	    "worker_nodes": ["127.0.0.1:6000"]
          },
          "layers": [
            {
              "name": "layer1",
              "data_source":"test_data_source",
              "start_isodate":"2001-01-01T00:00:00.000Z",
              "end_isodate":"2002-05-26T00:00:00.000Z",
              "step_days":1,
              "time_generator":"regular",
              "rgb_products":[
                 "bare_soil",
                 "phot_veg",
                 "nphot_veg"
              ]
            }
          ]
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

@test "ows trailing slash test" {
    run wget -O /dev/null --max-redirect 0 http://localhost:8080/ows
    [ "$status" -eq 0 ]
}

@test "ows max width height test" {
    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-06-18T00%3A00%3A00.000Z&srs=EPSG%3A3857&transparent=true&format=image%2Fpng&exceptions=application%2Fvnd.ogc.se_xml&tiled=true&feature_count=101&service=WMS&version=1.1.1&request=GetMap&layers=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=256&height=256'
    [ "$status" -eq 0 ]

    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-06-18T00%3A00%3A00.000Z&srs=EPSG%3A3857&transparent=true&format=image%2Fpng&exceptions=application%2Fvnd.ogc.se_xml&tiled=true&feature_count=101&service=WMS&version=1.1.1&request=GetMap&layers=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=256&height=513'
    [ "$status" -ne 0 ]

    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-06-18T00%3A00%3A00.000Z&srs=EPSG%3A3857&transparent=true&format=image%2Fpng&exceptions=application%2Fvnd.ogc.se_xml&tiled=true&feature_count=101&service=WMS&version=1.1.1&request=GetMap&layers=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=513&height=256'
    [ "$status" -ne 0 ]

    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-01-01T00%3A00%3A00.000Z&crs=EPSG%3A3857&format=geotiff&service=WCS&version=1.0.0&request=GetCoverage&coverage=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=512&height=256'
    [ "$status" -eq 0 ]

    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-01-01T00%3A00%3A00.000Z&crs=EPSG%3A3857&format=geotiff&service=WCS&version=1.0.0&request=GetCoverage&coverage=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=50001&height=256'
    [ "$status" -ne 0 ]

    run wget --quiet -O /dev/null 'http://127.0.0.1:8080/ows?time=2018-01-01T00%3A00%3A00.000Z&crs=EPSG%3A3857&format=geotiff&service=WCS&version=1.0.0&request=GetCoverage&coverage=global%3Ac6%3Afrac_cover&bbox=12523442.714243278%2C-10018754.17139462%2C15028131.257091936%2C-7514065.628545966&width=512&height=30001'
    [ "$status" -ne 0 ]

}
