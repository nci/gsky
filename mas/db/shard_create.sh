#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1
gpath=$2

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select true from ${shard}.paths limit 1;
EOD
) && echo "Shard '${shard}' existed. Skipping shard creation." && exit 1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
create schema if not exists ${shard};
set search_path to ${shard};
grant usage on schema ${shard} to public;
alter default privileges for role mas in schema ${shard} grant select on tables to public;

insert into public.shards (sh_code, sh_path)
  values ('${shard}', '${gpath}')
  on conflict (sh_code) do nothing;

\\i shard.sql

EOD
)
