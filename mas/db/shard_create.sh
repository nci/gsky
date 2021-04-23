#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1
gpath=$2

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select true from ${shard}.paths limit 1;
EOD
) && ret=$(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select 1 from public.shards where sh_code = '${shard}' and sh_path = '${gpath}' limit 1;
EOD
) && [ -z "$ret" ] && echo "Shard '${shard}' existed with different gpath." >&2 && exit 2

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select true from ${shard}.paths limit 1;
EOD
) && echo "Shard '${shard}' existed. Skipping shard creation." >&2 && exit 1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
create schema if not exists ${shard};
set search_path to ${shard},public;
grant usage on schema ${shard} to public;
alter default privileges for role mas in schema ${shard} grant select on tables to public;

insert into shards (sh_code, sh_path)
  values ('${shard}', '${gpath}')
  on conflict (sh_code) do nothing;

\\i shard.sql

grant select,insert,update on ${shard}.timestamps_cache to api;

EOD
)
