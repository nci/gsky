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
	file_list=$data_dir/${job_id}.filelist.gz

	set -ex
	find $find_dir -name "$file_pattern" $find_params | gzip > ${file_list}
	set +x
else
	set -eu

	file_list="$CRAWL_FILE_LIST"
	filename=$(basename $file_list)
	job_id=${filename%.*}
fi

crawl_file="$data_dir/${job_id}_gdal.tsv.gz"

total_files=$(cat $file_list | wc -l)
batch_size=$(echo "batch=${total_files}/${conc_limit}; if(batch<1){batch=1;}; if(batch>10){batch=10;}; batch;"|bc)

echo "INFO: file list to crawl: $file_list"
echo "INFO: crawl output file: $crawl_file"
echo "INFO: crawl batch size: $batch_size"

export GDAL_PAM_ENABLED=NO
export GDAL_NETCDF_VERIFY_DIMS=NO
CRAWL_EXTRA_ARGS=${CRAWL_EXTRA_ARGS:-''}

zcat $file_list | concurrent -i -l $conc_limit -b $batch_size $gsky_crawler - -fmt tsv $CRAWL_EXTRA_ARGS | gzip > $crawl_file
