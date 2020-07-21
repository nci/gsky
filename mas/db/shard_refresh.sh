#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
set search_path to ${shard}_tmp;

select refresh_polygons();
select refresh_caches();

set search_path to public;
alter schema ${shard} rename to ${shard}_old;
alter schema ${shard}_tmp rename to ${shard};
drop schema ${shard}_old cascade;
EOD
)
