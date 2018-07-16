#!/bin/bash
set -xeu

prefix=${PREFIX:-/usr}

v=10.4
(set -xeu
wget -q https://ftp.postgresql.org/pub/source/v10.4/postgresql-${v}.tar.gz
tar -xf postgresql-${v}.tar.gz
cd postgresql-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf postgresql-${v}
rm -f postgresql-${v}.tar.gz
