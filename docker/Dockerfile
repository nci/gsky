FROM ubuntu:18.04
MAINTAINER GSKY Developers <help@nci.org.au>

RUN apt-get update \
      && apt-get install -y --no-install-recommends \
        ca-certificates libreadline-dev cmake openssl curl wget git bc \
        pkg-config unzip autoconf automake libtool build-essential bison flex vim less

COPY ./build_deps.sh /
RUN ./build_deps.sh

COPY ./build_pgsql.sh /
RUN ./build_pgsql.sh

COPY ./build_gdal.sh /
RUN ./build_gdal.sh

COPY ./build_postgis.sh /
RUN ./build_postgis.sh

COPY ./build_terriajs.sh /
RUN ./build_terriajs.sh

ARG gsky_repo
COPY ./build_gsky.sh /
RUN ./build_gsky.sh "$gsky_repo"

COPY ./setup_mas.sh /
RUN ./setup_mas.sh

ARG sample_data_dir=./sample_data
ADD "${sample_data_dir}" /gdata

COPY "./gsky_config.json" /gsky/etc/config.json

COPY ./ingest_sample_data.sh /
RUN ./ingest_sample_data.sh

# TerriaJS requires a proxy server to provide certain RESTful services,
# or an error message box will pop up during app startup.
# (details: https://github.com/TerriaJS/terriajs/blob/master/lib/Models/Terria.js#L508)
# We bypass this issue by creating dummy URLs with empty JSON responses.
RUN mkdir /gsky/share/gsky/static/terria/serverconfig \
      && echo '{}' > /gsky/share/gsky/static/terria/serverconfig/index.html \
      && mkdir /gsky/share/gsky/static/terria/proxyabledomains \
      && echo '{}' > /gsky/share/gsky/static/terria/proxyabledomains/index.html

RUN rm /gsky/share/gsky/static/terria/init/*.json
COPY terria_init.json /gsky/share/gsky/static/terria/init/terria.json
COPY terria_config.json /gsky/share/gsky/static/terria/config.json

COPY ./wps_payload.xml /
COPY ./demo_wps_request.sh /

EXPOSE 8080
EXPOSE 8888

COPY ./gsky_entry_point.sh /
ENTRYPOINT ./gsky_entry_point.sh
