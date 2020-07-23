#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

set role mas;
drop schema if exists ${shard}_tmp cascade;
create schema if not exists ${shard}_tmp;
set search_path to ${shard}_tmp;

grant usage on schema ${shard}_tmp to public;
alter default privileges for role mas in schema ${shard}_tmp grant select on tables to public;

\\i shard.sql

grant select,insert,update on ${shard}_tmp.timestamps_cache to api;

EOD
)
