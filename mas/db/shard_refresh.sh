#!/bin/bash

## This file is now only for compatibility of old ingestion pipeline code

#here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
#shard=$1

#(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

#set role mas;
#set search_path to ${shard};

#select refresh_polygons();
#select refresh_caches();

#EOD
#)
