set -eu

export PATH="$PATH:/usr/bin"
export LD_LIBRARY_PATH="${LD_LIBRARY_PATH-:}:/usr/local/lib"

export PGUSER=postgres
export PGDATA=/pg_data

useradd $PGUSER
mkdir $PGDATA
chown $PGUSER:$PGUSER $PGDATA

su -p -c "initdb -A trust -U $PGUSER" -l $PGUSER
su -p -c "pg_ctl -w start" -l $PGUSER

(cd /gsky/share/mas && psql -f /gsky/share/mas/schema.sql)
(cd /gsky/share/mas && psql -f /gsky/share/mas/api/mas.sql)

su -p -c "pg_ctl -w stop" -l $PGUSER
