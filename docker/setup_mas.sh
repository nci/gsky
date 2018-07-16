#!/bin/bash
set -xeu

export PATH="$PATH:/usr/bin"
export LD_LIBRARY_PATH="${LD_LIBRARY_PATH-:}:/usr/local/lib"

export PGUSER=postgres
export PGDATA=/pg_data

useradd $PGUSER
mkdir $PGDATA
chown $PGUSER:$PGUSER $PGDATA

su -p -c "initdb -A trust -U $PGUSER" -l $PGUSER
su -p -c "pg_ctl -w start" -l $PGUSER

ls /gsky/share/mas
(cd /gsky/share/mas && psql -f schema.sql)
(cd /gsky/share/mas && psql -f mas.sql)

su -p -c "pg_ctl -w stop" -l $PGUSER
