set -eu

prefix=${PREFIX:-/usr}

v=10.4
wget -q https://ftp.postgresql.org/pub/source/v10.4/postgresql-${v}.tar.gz
tar -xf postgresql-${v}.tar.gz
cd postgresql-${v}
./configure --prefix=$prefix
make -j4
make install
cd ..
rm -rf postgresql-${v}
