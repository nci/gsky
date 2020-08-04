#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

psql -v ON_ERROR_STOP=1 -A -t -q -d mas \
  -c "select true from ${shard}_tmp.paths limit 1;" \
  || (cd "$here" && ./shard_reset.sh "$shard")

batch_size=${INGEST_BATCH_SIZE:-10000}
conc_limit=${INGEST_CONC_LIMIT:-$(nproc --all)}
(cd "$here" && concurrent -v -l $conc_limit -b $batch_size -r 3 ./ingest.sh "${shard}_tmp")
