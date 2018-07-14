#!/bin/bash
set -xeu

export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

export PGUSER=${PGUSER:-postgres}
export PGDATA=${PGDATA:-/pg_data}

masapi_port=8888
rpc_port=6000
ows_port=8080

su -p -c "pg_ctl -w start" -l "$PGUSER"

# masapi requires postgresql unix domain socket under /var/run
mkdir -p /var/run/postgresql
ln -s /tmp/.s.PGSQL.5432 /var/run/postgresql/.s.PGSQL.5432 

./gsky/bin/masapi -port $masapi_port -pool 2 2>&1 > masapi_output.log &

n_cores=$(grep '^cpu\s*cores' /proc/cpuinfo|uniq|awk '{print $4}')
if [[ ! "$n_cores" =~ ^[0-9]+$ ]]
then
  echo 'cannot determine number of cpu cores, using default value 1'
  n_cores=1
elif [ $n_cores -lt 2 ]
then
  n_cores=1
fi

./gsky/bin/gsky-rpc -p $rpc_port -n $n_cores 2>&1 > rpc_output.log &
./gsky/bin/gsky-ows -p $ows_port &

set +x
sleep 0.5
echo
echo
echo '=========================================================='
echo 'Welcome to GSKY Docker'
echo
echo "TerriaMap:               http://127.0.0.1:$ows_port/terria"
echo "GSKY WMS/WCS end point:  http://127.0.0.1:$ows_port/ows"
echo "MAS API end point:       http://127.0.0.1:$masapi_port"
echo
echo '=========================================================='

wait
