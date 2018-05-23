#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && runuser postgres -c 'psql -v ON_ERROR_STOP=1 -A -t -q -d mas' <<EOD
  select true from ${shard}.paths limit 1
EOD
)
