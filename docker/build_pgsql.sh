#!/bin/bash
set -xeu

prefix=${PREFIX:-/usr}

v=11.5
(set -xeu
wget -q https://ftp.postgresql.org/pub/source/v${v}/postgresql-${v}.tar.gz
tar -xf postgresql-${v}.tar.gz
cd postgresql-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf postgresql-${v}
rm -f postgresql-${v}.tar.gz
