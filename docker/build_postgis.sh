set -eu

#export PATH=/usr/local/pgsql/bin:$PATH
#export LD_LIBRARY_PATH=/usr/local/pgsql/lib

v=2.4.4
wget -q https://download.osgeo.org/postgis/source/postgis-${v}.tar.gz
tar -xf postgis-${v}.tar.gz
cd postgis-${v}
make -j4
make install
cd ..
