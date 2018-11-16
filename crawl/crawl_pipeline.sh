#!/bin/bash
set -e
which concurrent
which gsky-crawl
set +e
crawl_job_id='u39'  # AVS

conc_limit=${CRAWL_CONC_LIMIT:-16}

if [ -z "$CRAWL_OUTPUT_DIR" ]
then
	here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
	data_dir=$here/crawl_tsv/`date +'%Y-%m-%d_%H-%M-%S'`
else
	data_dir="$CRAWL_OUTPUT_DIR"
fi

#mkdir -p $data_dir # AVS commented out

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
#	file_list=$data_dir/${job_id}.filelist
	file_list=$data_dir/${crawl_job_id}.filelist # AVS

	set +ex # AVS changed from -ex to +ex
	find $find_dir -name "$file_pattern" $find_params > ${file_list}
	set +x
else
	set -eu

	file_list="$CRAWL_FILE_LIST"
	filename=$(basename $file_list)
	job_id=${filename%.*}
fi
#crawl_file="$data_dir/${job_id}_gdal.tsv.gz"
crawl_file="$data_dir/${crawl_job_id}_gdal.tsv.gz" # AVS

echo "INFO: file list to crawl: $file_list"
echo "INFO: crawl output file: $crawl_file"

gdal_json() {
	src_file="$1"
	json=$(gsky-crawl $src_file)
	echo -e "$src_file\tgdal\t$json"
}
export -f gdal_json
export GDAL_PAM_ENABLED=NO
#cat $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $crawl_file
tail $file_list | concurrent -i -l $conc_limit xargs bash -c 'gdal_json "$@"' _ | gzip > $crawl_file

