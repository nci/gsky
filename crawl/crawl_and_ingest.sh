#!/bin/bash
#-------------------------------------------------------------------------------
# crawl_and_ingest.sh
# To read and ingest the *.nc files
# Created on 19 Nov, 2018 by Arapaut V Sivaprasad
# Adapted from various scripts created by Edison Guo
# Last modified on: 19 Nov 2018
#-------------------------------------------------------------------------------
# SUMMARY OF OPERATION
#	- Add the 'gsky' paths to $PATH
#	- Add '/usr/local/lib' to $LD_LIBRARY_PATH
#	- Set the maximum number of concurrent processes. Can go up to 8x the no. of CPUs
#	- Define environment variables: setup_crawling()
#		- $CRAWL_DIR for input files in *.nc format
#		- $CRAWL_OUTPUT_DIR - output location for temporary *.filelist and *.tsv files 
#		- $PATH and $LD_LIBRARY_PATH
#		- $CRAWL_CONC_LIMIT as number of parallel processes
#		- $PGUSER - to enable the execution of PSQL 
#		- $PGDATA - the location of PSQL data files.
#		- $PGHOST - PostgreSQL database hostname
#		- $PGPORT - PostgreSQL database port
#		- $GPATH  - Derived from $CRAWL_DIR
#		- $SHARD  - Schema name for Postgres database. Generaly the sub-dir under /g/data2. e.g. 'tc43'
#		- Check the input dir for *.nc files
#		- Delete all files in $CRAWL_OUTPUT_DIR and re-create this directory
#	- Call gdal_json() to crawl the *.nc files using 'gsky-crawl'
#	- Create a postgreSQL database schema by the name, $SHARD
#	- Unzip the crawled and gzipped TSV file
#	- Ingest into the database table, 'ingest', using the '\copy' command
#	- Zip and keep the crawled TSV file
#-------------------------------------------------------------------------------

export PATH="/local/gsky/bin:/local/gsky/share/mas:/local/gsky/share/gsky:$PATH"
export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"
export CRAWL_CONC_LIMIT=4

export CRAWL_DIR=/g/data2/tc43/modis-fc/v310/tiles/8-day/cover/
# Specify and uncomment below if required to have a fixed dir for output files.
#export CRAWL_OUTPUT_DIR=/home/900/avs900/crawl_outputs 

# Specify below a list of files having full pathnames. 
# e.g. /g/data2/tc43/modis-fc/v310/tiles/8-day/cover/FC.v310.MCD43A4.h23v09.2008.006.nc
#export CRAWL_FILE_LIST=/home/900/avs900/crawl_files.txt 

# Take the first 3 dir paths in $CRAWL_DIR as $GPATH. e.g. /g/data2/tc43
IFS="/" read -r -a array <<< $CRAWL_DIR
gpath="/${array[1]}/${array[2]}/${array[3]}"
shard="${array[3]}"
export GPATH=$gpath # /g/data2/tc43
export SHARD=$shard # tc43

export PGUSER=postgres
export PGDATA=/usr/local/pgsql/data
export PGHOST=${PGHOST:-''}
export PGPORT=${PGPORT:-5432}

# ------ START of Functions ------------------
setup_mas() {
psql -v ON_ERROR_STOP=1 -A -t -q <<EOD
\\i /local/gsky/share/mas/mas.sql
EOD
}
setup_crawling() {
	# Exit if these two executables are not in $PATH. The '-xeu' will abort the script.
	set -e
	which concurrent
	which gsky-crawl
	set +e
	# Create the $CRAWL_OUTPUT_DIR, if not specified, as a sub-dir of the script dir. 
	if [ ! "$CRAWL_OUTPUT_DIR" ]
	then
		here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
		CRAWL_OUTPUT_DIR=$here/crawl_tsv/`date +'%Y-%m-%d_%H-%M-%S'`
		export CRAWL_OUTPUT_DIR=$CRAWL_OUTPUT_DIR
	fi
	
	# Just to be safe, delete and re-create the $CRAWL_OUTPUT_DIR. This is not necessary.
	rm -rf $CRAWL_OUTPUT_DIR 
	mkdir -p $CRAWL_OUTPUT_DIR 
	
	# Check whether the $CRAWL_DIR has any *.nc files. If not, check if $CRAWL_FILE_LIST has.
	res=$(find "$CRAWL_DIR" -name "*.nc")
	if [ -z "$res" ]
	then
		echo "CRAWL_DIR is not defined and/or there is no *.nc files under '$CRAWL_DIR'."
		if [ -z "$CRAWL_FILE_LIST" ]
		then
			echo "CRAWL_FILE_LIST is not defined and/or there is no file list in '$CRAWL_FILE_LIST'."
			exit 1
		else
			file_list=$CRAWL_FILE_LIST
			filename=$(basename $file_list)
			job_id=${filename%.*}
		fi
	else
		file_pattern=${CRAWL_PATTERN:-*.nc} # Is this necessary?
		find_params=${CRAWL_PARAMS} # Specify params to the 'find' command. Is it necessary?
	
		job_id="${CRAWL_DIR//[\/]/_}"
		file_list=$CRAWL_OUTPUT_DIR/${job_id}.filelist # AVS
		find $CRAWL_DIR -name "$file_pattern" $find_params > ${file_list}	
	fi
	tsv_file_gz="$CRAWL_OUTPUT_DIR/${job_id}_gdal.tsv.gz"
}
gdal_json() {
	src_file="$1"
	json=$(gsky-crawl $src_file)
	echo -e "$src_file\tgdal\t$json"
}
# ------ END of Functions ------------------

# Create the 'mas' database. No need to redo it, but no harm in it either. But, DO NOT run 'schema.sql' here.
setup_mas
# Check for directories, file_list, and tsv_file_gz. Create them if necessary.
setup_crawling 
echo "INFO: file list to crawl: $file_list"
echo "INFO: crawl output file: $tsv_file_gz"

# -x: display commands; -e: exit on error; -u: unset var is an error
set -xeu

# Crawl the dir and create the TSV file and gzip it.
export -f gdal_json
export GDAL_PAM_ENABLED=NO
conc_limit=${CRAWL_CONC_LIMIT:-16}

#cat $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $tsv_file_gz
tail $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $tsv_file_gz

# Create the PostgreSQL database schema
psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD
set role mas;
create schema if not exists "$SHARD";
set search_path to "$SHARD";
grant usage on schema "$SHARD" to public;
alter default privileges for role mas in schema "$SHARD" grant select on tables to public;

insert into public.shards (sh_code, sh_path) values ('$SHARD', '$GPATH') on conflict (sh_code) do nothing;

\\i /local/gsky/share/mas/shard.sql	

grant select,insert,update on "$SHARD".timestamps_cache to api;

EOD

# Unzip the gzipped TSV file
res=`gunzip $tsv_file_gz`
tsv_file=${tsv_file_gz/.gz/}

# Ingest the lines in $tsv_file into the DB table, 'ingest'.
psql -v ON_ERROR_STOP=1 -d mas <<EOD
set search_path to "$SHARD";
drop trigger if exists ingest on ingest cascade;
drop trigger if exists ingested on ingest cascade;
\copy ingest from '$tsv_file' with (format 'csv', delimiter E'\t', quote E'\b');
select refresh_views();
select refresh_polygons();
select refresh_caches();
EOD

# Zip the gzipped TSV file. Is there any need to retain it?
res=`gzip $tsv_file`
echo "--------------- FINISHED ! ----------------"
