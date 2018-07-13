set -xeu

export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

export PGUSER=${PGUSER:-postgres}
export PGDATA=${PGDATA:-/pg_data}

su -p -c "pg_ctl -w start" -l $PGUSER
mkdir -p /var/run/postgresql
ln -s /tmp/.s.PGSQL.5432 /var/run/postgresql/.s.PGSQL.5432 

./gsky/bin/masapi -port 8888 -pool 2 2>&1 > masapi_output.log &
./gsky/bin/gsky-rpc -p 6000 -n 2 2>&1 > rpc_output.log &
./gsky/bin/gsky-ows -p 8080
