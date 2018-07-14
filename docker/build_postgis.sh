#!/bin/bash
set -xeu

v=2.4.4
(set -xeu
wget -q https://download.osgeo.org/postgis/source/postgis-${v}.tar.gz
tar -xf postgis-${v}.tar.gz
cd postgis-${v}
make -j4
make install
)
rm -rf postgis-${v}
rm -f postgis-${v}.tar.gz
