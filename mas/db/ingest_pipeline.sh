#!/bin/bash
set -u
set -e
which concurrent
set +e

if [ "$#" -lt 2 ]
then
	echo 'USAGE: ingest_pipeline.sh <shard> <crawl file1> ... <crawl fileN>'
	exit 127
fi

shard="$1"
craw_file="$2"

gpath=${GPATH:-''}

get_gpath () {
	crawl_res=$(zcat "$1" |tail -n 1)
	IFS=$'\t' read -r -a crawl_parts <<< "$crawl_res"
	if [ ${#crawl_parts[@]} -ne 3 ]
	then
		return 1
	fi

	raw_data_path=$(dirname "${crawl_parts[0]}")

	data_path=$(echo "$raw_data_path" | sed -r 's/\/{2,}/\//g')

	IFS="/" read -r -a path_parts <<< "$data_path"

	if [ ${#path_parts[@]} -eq 0 ]
	then
		return 2
		#echo 'no data path is specified' && exit 1
	fi

	gpath=''
	n_folders=0
	for idx in "${!path_parts[@]}"
	do
		folder=${path_parts[$idx]}

		if [ -n "$folder" ]
		then
			gpath="$gpath/$folder"
			n_folders=$((n_folders+1))
		fi

		if [ $n_folders -eq 3 ]
		then
			break
		fi
	done

	echo "$gpath"
	return 0

}

assert_gpath () {
	if [ ! "$1" -eq 0 ]
	then	
		if [ "$1" -eq 1 ]
		then
			echo 'no crawl results or invalid format of crawl results' 
		elif [ "$1" -eq 2 ]
		then
			echo 'no data path is specified' 
		else
			echo 'unknown error'
		fi
		exit "$1"
	fi

	return "$1"
}

if [ -z "$gpath" ]
then
	gpath=$(get_gpath "$craw_file")
	assert_gpath $?
fi

idx=0
for crawl_file in "$@"
do
	idx=$((idx+1))
	if [ $idx -eq 1 ]
	then
		continue
	fi

	gpath_other=$(get_gpath "$crawl_file")
	if [ "$gpath_other" != "$gpath" ]
	then
		echo "FATAL: different gpath, first gpath: ${gpath}, other gpath: ${gpath_other}"
		exit 4
	fi
done

export PGUSER=${PGUSER:-postgres}
export PGHOST=${PGHOST:-''}
export PGPORT=${PGPORT:-5432}

echo "INFO: pg_user=$PGUSER pg_host=$PGHOST pg_port=$PGPORT gpath=$gpath"

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
(cd "$here" && bash shard_create.sh "${shard}" "${gpath}")

ret_code=$?
if [ $ret_code -ne 0 ] && [ $ret_code -ne 1 ]
then
	echo 'FATAL: shard_create.sh failed'
	exit $ret_code 
fi

set -e

idx=0
for crawl_file in "$@"
do
	idx=$((idx+1))
	if [ $idx -eq 1 ]
	then
		continue
	fi

	abs_filepath=$(readlink -f "$crawl_file")
	echo "INFO: ingesting $abs_filepath"

	(cd "$here" && zcat "${abs_filepath}" | bash shard_ingest.sh "${shard}")
done

(cd "$here" && bash shard_refresh.sh "${shard}")
