#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && ./shard_reset.sh "$shard")

(cd "$here" && concurrent -v -b 10000 -r 3 ./ingest.sh "${shard}_tmp")
