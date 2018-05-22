#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

shard=$1

pushd $here

runuser postgres -c "concurrent -v -b 10000 -r 3 ./ingest.sh ${shard}"

popd
