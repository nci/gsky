#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
drop schema if exists ${shard}_tmp cascade;
create schema if not exists ${shard}_tmp;

EOD
)
