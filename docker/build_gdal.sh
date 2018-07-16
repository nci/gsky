#!/bin/bash
set -xeu

v=2.3.1
(set -xeu
wget -q http://download.osgeo.org/gdal/${v}/gdal-${v}.tar.gz
tar -xf gdal-${v}.tar.gz
cd gdal-${v}
./configure --with-geos=yes --with-netcdf
make -j4
make install
)
rm -rf gdal-${v}
rm -f gdal-${v}.tar.gz
