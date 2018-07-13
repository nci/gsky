set -xeu

export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

./gsky/bin/masapi -port 8888 &
./gsky/bin/gsky-rpc -p 6000 &
./gsky/bin/gsky-ows -p 8080 &

echo 'GSKY is listening at ::8080'
wait
