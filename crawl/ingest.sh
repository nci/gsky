#!/bin/bash
#-------------------------------------------------------------------------------
# ingest.sh
# To ingest the TSV file created by 'crawl.sh'
# Created on 21 Nov, 2018 by Arapaut V Sivaprasad
# Adapted from various scripts created by Jian Edison Guo
# Last modified on: 21 Nov 2018
#-------------------------------------------------------------------------------
# SUMMARY OF OPERATION
#	- Create a postgreSQL database schema by the name, $SHARD
#	- Unzip the crawled and gzipped TSV file
#	- Ingest into the database table, 'ingest', using the '\copy' command
#	- Zip and keep the crawled TSV file
#-------------------------------------------------------------------------------
# USAGE:
# 1. Run this script AFTER executing 'crawl.sh'
#   	e.g. 'source /.../gsky/crawl/crawl.sh; /.../gsky/crawl/ingest.sh' 
#		NOTE: It is important to 'source' the crawl.sh but not ingest.sh
# 2. Watch for errors, if any.
# 3. Connect to PSQL and check that the data has been added to the DB
# 		psql -d mas
# 		set search_path to "tc43"; # (or whatever in $SHARD)
# 		select count(*) from metadata; 
#-------------------------------------------------------------------------------
# These environment variables are exported from crawl.sh
shard=$SHARD
gpath=$GPATH
tsv_file_gz=$tsv_file_gz 

# Check whether a schema named $shard exists. 
(
psql -v ON_ERROR_STOP=0 -A -t -q -d mas <<EOD
select true from ${shard}.paths limit 1;
EOD
) && ret=$(psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select 1 from public.shards where sh_code = '${shard}' and sh_path = '${gpath}' limit 1;
EOD
) && [ -z "$ret" ] && export shard_exists=""

(
psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
select true from ${shard}.paths limit 1;
EOD
) && export shard_exists="Yes" 
	
# Create a schema named $SHARD, if it does not exist, and create the tables and functions	
if [ -z $shard_exists ]
then
psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
set role mas;
create schema if not exists "$shard";
set search_path to "$shard";
grant usage on schema "$shard" to public;
alter default privileges for role mas in schema "$shard" grant select on tables to public;

insert into public.shards (sh_code, sh_path) values ('$shard', '$gpath') on conflict (sh_code) do nothing;

\\i /local/gsky/share/mas/shard.sql	

grant select,insert,update on "$shard".timestamps_cache to api;

EOD
fi
#-------------------------------------------------------------------------------
# Ingest the data from TSV
# Unzip the gzipped TSV file
res=`gunzip $tsv_file_gz`
tsv_file=${tsv_file_gz/.gz/}

# Ingest the lines in $tsv_file into the DB tables.
echo "# Ingest the lines in TSV file into the DB tables."
psql -v ON_ERROR_STOP=1 -d mas <<EOD
set search_path to "$SHARD";
\copy ingest from '$tsv_file' with (format 'csv', delimiter E'\t', quote E'\b');
select refresh_views();
select refresh_polygons();
select refresh_caches();
select count(*) as "n_records ingested" from metadata; 
select md_hash as "md_hash in metadata" from metadata limit 50; 
select pa_path as "pa_path in paths" from paths limit 50; 
select po_hash as "po_hash in polygons" from polygons limit 50;
EOD

# Zip the gzipped TSV file. Is there any need to retain it?
res=`gzip $tsv_file`
echo "--------------- FINISHED ingesting ! ----------------"
