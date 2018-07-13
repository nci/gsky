set -xe

v=2.3.1
wget -q http://download.osgeo.org/gdal/${v}/gdal-${v}.tar.gz
tar -xf gdal-${v}.tar.gz
cd gdal-${v}
./configure --with-geos=yes --with-netcdf
make -j4
make install
cd ..
rm -rf gdal-${v}
