FROM ubuntu:18.10 
MAINTAINER GSKY Developers <help@nci.org.au>

RUN apt-get update \
      && apt-get install -y --no-install-recommends \
        ca-certificates libreadline-dev cmake openssl wget git \
        pkg-config unzip autoconf build-essential bison flex

COPY ./build_deps.sh /
RUN ./build_deps.sh

COPY ./build_pgsql.sh /
RUN ./build_pgsql.sh

COPY ./build_gdal.sh /
RUN ./build_gdal.sh

COPY ./build_postgis.sh /
RUN ./build_postgis.sh

COPY ./build_gsky.sh /
RUN ./build_gsky.sh

COPY ./mas.sql /gsky/share/mas/api/mas.sql
COPY ./setup_mas.sh /
RUN ./setup_mas.sh

ARG sample_data_dir=./sample_data
ADD "${sample_data_dir}" /gdata

ARG sample_config_dir=./sample_config
COPY "${sample_config_dir}/config.json" /gsky/etc/

COPY ./ingest_sample_data.sh /
RUN ./ingest_sample_data.sh

COPY ./setup_terriajs.sh /
RUN ./setup_terriajs.sh

EXPOSE 8080

COPY ./gsky_entry_point.sh /
ENTRYPOINT ./gsky_entry_point.sh
