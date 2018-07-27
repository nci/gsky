set -xe
curl -X POST -d "@wps_payload.xml" http://127.0.0.1:8080/ows?service=WPS&request=Execute
