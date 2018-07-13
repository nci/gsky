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

COPY ./setup_mas.sh /
RUN ./setup_mas.sh

ARG sample_data_dir=./sample_data
ADD "$sample_data_dir" /gdata
COPY $sample_data_dir"/config.json /gsky/etc/

COPY ./ingest_sample_data.sh /
RUN ./ingest_sample_data.sh

COPY ./gsky_entry_point.sh /
RUN ./gsky_entry_point.sh
