#!/bin/bash

shard=$1

iconv -f ISO-8859-1 -t UTF-8 | sort | uniq | psql -v ON_ERROR_STOP=1 -A -t -q -d mas \
  -c "set search_path to ${shard},public; copy ingest from stdin with (format 'csv', delimiter E'\\t', quote E'\\b');" >/dev/null
