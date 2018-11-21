#!/bin/bash
#-------------------------------------------------------------------------------
# crawl.sh
# To crawl the *.nc files
# Created on 19 Nov, 2018 by Arapaut V Sivaprasad
# Adapted from various scripts created by Jian Edison Guo
# Last modified on: 21 Nov 2018
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
#-------------------------------------------------------------------------------
# USAGE:
# 1. Edit this script to insert the value for CRAWL_DIR and optionally for CRAWL_OUTPUT_DIR
# 2. Execute it as e.g. 'source /.../gsky/crawl/crawl.sh' 
#	NOTE: You must execute the script with the 'source' command. 
#		  Just running it as ./crawl.sh will not export the required variables 
#		  to the parent shell to be used by the next script, 'ingest.sh'
#-------------------------------------------------------------------------------

export PATH="/local/gsky/bin:/local/gsky/share/mas:/local/gsky/share/gsky:$PATH"
export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"
export CRAWL_CONC_LIMIT=32

export CRAWL_DIR=/g/data2/tc43/modis-fc/v310/tiles/8-day/cover
#export CRAWL_DIR=/g/data2/tc43/rainfall/CHIRPS-2.0/global_dekad/netcdf
#export CRAWL_DIR=/g/data2/tc43/rainfall/CHIRPS-2.0/global_monthly/netcdf

# Specify and uncomment below if required to have a fixed dir for output files.
#export CRAWL_OUTPUT_DIR=/home/900/avs900/crawl_outputs 

# Specify below a list of files having full pathnames. 
# e.g. /g/data2/tc43/modis-fc/v310/tiles/8-day/cover/FC.v310.MCD43A4.h23v09.2008.006.nc
#export CRAWL_FILE_LIST=/home/900/avs900/crawl_files.txt 

# Take the first 3 dir paths in $CRAWL_DIR as $GPATH. e.g. /g/data2/tc43
IFS="/" read -r -a array <<< $CRAWL_DIR
gpath="/${array[1]}/${array[2]}/${array[3]}"
export GPATH=$gpath # /g/data2/tc43
shard="${array[3]}"
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
	# Exit if these two executables are not in $PATH. The '-eu' will abort the script.
	set -eu
	which concurrent
	which gsky-crawl
	set +eu
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
		file_pattern=${CRAWL_PATTERN:-*.nc} # Search for *.nc files
		find_params=${CRAWL_PARAMS} # Specify params to the 'find' command. Is it necessary?
	
		job_id="${CRAWL_DIR//[\/]/_}"
		file_list=$CRAWL_OUTPUT_DIR/${job_id}.filelist 
		find $CRAWL_DIR -name "$file_pattern" $find_params > ${file_list}	
	fi
	tsv_file_gz="$CRAWL_OUTPUT_DIR/${job_id}_gdal.tsv.gz"
	export tsv_file_gz=$tsv_file_gz
}
gdal_json() {
	json=$(gsky-crawl $1)
	echo -e "$1\tgdal\t$json"
}
# ------ END of Functions ------------------

# Uncomment the line below if database, mas, has not been setup.
#setup_mas

# Check for directories, file_list, and tsv_file_gz. Create them if necessary.
setup_crawling 
echo "INFO: file list to crawl: $file_list"
echo "INFO: crawl output file: $tsv_file_gz"

# -x: display commands; -e: exit on error; -u: unset var is an error
#set -xeu

# Crawl the dir and create the TSV file and gzip it.
export -f gdal_json
export GDAL_PAM_ENABLED=NO
conc_limit=${CRAWL_CONC_LIMIT:-16}

#cat $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $tsv_file_gz
tail -5 $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $tsv_file_gz

#export CRAWL_OUTPUT_DIR=$CRAWL_OUTPUT_DIR
ls -l $CRAWL_OUTPUT_DIR
echo "----- FINISHED Crawling ! -----
You must run the 'ingest.sh' (uncomment below) to read the TSV file into the database."
./ingest.sh
