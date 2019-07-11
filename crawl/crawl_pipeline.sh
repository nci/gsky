#!/bin/bash
gsky_crawler=${CRAWL_BIN:-gsky-crawl}
set -e
which concurrent
[[ "$gsky_crawler" = "gsky-crawl" ]] && which gsky-crawl
set +e

conc_limit=${CRAWL_CONC_LIMIT:-16}

if [ -z "$CRAWL_OUTPUT_DIR" ]
then
	here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
	data_dir=$here/crawl_tsv/`date +'%Y-%m-%d_%H-%M-%S'`
else
	data_dir="$CRAWL_OUTPUT_DIR"
fi

mkdir -p $data_dir

if [ -z "$CRAWL_FILE_LIST" ]
then
	if [ -z "$CRAWL_DIR" ]
	then
		echo 'Please set environment variable $CRAWL_DIR to specify the directory to crawl, or set $CRAWL_FILE_LIST to supply a file list'
		exit 1
	fi

	find_dir="$CRAWL_DIR"
	file_pattern=${CRAWL_PATTERN:-*.nc}
	find_params=${CRAWL_PARAMS}

	set -u
	
	job_id="${find_dir//[\/]/_}"
	file_list=$data_dir/${job_id}.filelist

	set -ex
	find $find_dir -name "$file_pattern" $find_params > ${file_list}
	set +x
else
	set -eu

	file_list="$CRAWL_FILE_LIST"
	filename=$(basename $file_list)
	job_id=${filename%.*}
fi

crawl_file="$data_dir/${job_id}_gdal.tsv.gz"

echo "INFO: file list to crawl: $file_list"
echo "INFO: crawl output file: $crawl_file"

gdal_json() {
	set -e
	src_file="$1"
	json=$($gsky_crawler $src_file $CRAWL_EXTRA_ARGS)
	[[ -z "$json" ]] && exit 1
	if [ -z "$CRAWL_RAW_RECORD" ]
	then
		echo -e "$src_file\tgdal\t$json"
	else
		echo -e "$json"
	fi
}

export -f gdal_json
export gsky_crawler=$gsky_crawler
export GDAL_PAM_ENABLED=NO
export GDAL_NETCDF_VERIFY_DIMS=NO

cat $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $crawl_file
