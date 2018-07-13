set -xeu

export PATH="/gsky/bin:/gsky/share/mas:$PATH"
export CRAWL_DIR=/gdata
export CRAWL_OUTPUT_DIR=/crawl_outputs
export CRAWL_CONC_LIMIT=2
export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

export PGUSER=postgres
export PGDATA=/pg_data

mkdir -p $CRAWL_OUTPUT_DIR

./gsky/bin/gsky-crawl_pipeline.sh

su -p -c "pg_ctl -w start" -l $PGUSER
(cd /gsky/share/mas && ./ingest_pipeline.sh u39 /crawl_outputs/_gdata_gdal.tsv.gz)
