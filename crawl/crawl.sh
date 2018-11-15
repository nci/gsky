#!/bin/bash
set -xeu

export PATH="/local/gsky/bin:/local/gsky/share/mas:/local/gsky/share/gsky:$PATH"
export CRAWL_DIR=/g/data2/tc43/modis-fc/v310/tiles/8-day/cover/
export CRAWL_OUTPUT_DIR=/home/900/avs900/crawl_outputs
export CRAWL_CONC_LIMIT=2
export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

export PGUSER=postgres
export PGDATA=/usr/local/pgsql/data
export GPATH=/g/data2/tc43

set +xe # AVS changed from +x to +xe
res=$(find "$CRAWL_DIR" -name "*.nc")
if [ -z "$res" ]
then
  echo "No *.nc files under '$CRAWL_DIR'."
  exit 1
fi
set -x

#rm -rf $CRAWL_OUTPUT_DIR # AVS commented out
#mkdir -p $CRAWL_OUTPUT_DIR # AVS commented out

#/local/gsky/share/gsky/crawl_pipeline.sh
/home/900/avs900/gsky/crawl/crawl_pipeline.sh

#crawl_job_id="${CRAWL_DIR//[\/]/_}"
crawl_job_id='u39' # AVS
#su -p -c "pg_ctl -w start" -l $PGUSER
(cd /local/gsky/share/mas && ./ingest_pipeline.sh $crawl_job_id $CRAWL_OUTPUT_DIR/${crawl_job_id}_gdal.tsv.gz)
#su -p -c "pg_ctl -w stop" -l $PGUSER
