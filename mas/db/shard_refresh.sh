#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

function cleanup()
{
(cd "$here" && psql -v -A -t -q -d mas <<EOD
drop schema if exists ${shard}_tmp cascade;
drop schema if exists ${shard}_old cascade;
EOD
)
}
trap cleanup EXIT

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
set search_path to ${shard}_tmp,public;

select refresh_polygons();
select refresh_caches();
select refresh_codegens();

set search_path to public;
alter schema ${shard} rename to ${shard}_old;
alter schema ${shard}_tmp rename to ${shard};
EOD
)
