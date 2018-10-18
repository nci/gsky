#!/bin/bash
set -xeu
prefix=${PREFIX:-/usr}

v=9c
(set -xeu
wget -q http://www.ijg.org/files/jpegsrc.v${v}.tar.gz
tar -xf jpegsrc.v${v}.tar.gz
cd jpeg-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf jpeg-${v}
rm -f jpegsrc.v${v}.tar.gz

v=2.3.0
(set -xeu
wget -q -O openjpeg-v${v}.tar.gz https://github.com/uclouvain/openjpeg/archive/v${v}.tar.gz
tar -xf openjpeg-v${v}.tar.gz
cd openjpeg-${v}
mkdir build
cd build
cmake .. -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="$prefix"
make -j4
make install
)
rm -rf openjpeg-${v}
rm -f openjpeg-v${v}.tar.gz

v=3.6.2
(set -xeu
wget -q http://download.osgeo.org/geos/geos-${v}.tar.bz2
bunzip2 geos-${v}.tar.bz2
tar -xf  geos-${v}.tar
cd geos-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf geos-${v}
rm -f geos-${v}.tar

v=5.1.0
vd=1.7
(set -xeu
wget -q http://download.osgeo.org/proj/proj-${v}.tar.gz
tar -xf proj-${v}.tar.gz
wget -q http://download.osgeo.org/proj/proj-datumgrid-${vd}.zip
unzip proj-datumgrid-${vd}.zip -d proj-${v}/nad/
cd proj-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf proj-${v}
rm -f proj-${v}.tar

v=1.2.8
(set -xeu
wget -q ftp://ftp.unidata.ucar.edu/pub/netcdf/netcdf-4/zlib-${v}.tar.gz
tar -xf zlib-${v}.tar.gz && cd zlib-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf zlib-${v}
rm -f zlib-${v}.tar.gz

v=4.2.13
(set -xeu
wget -q https://support.hdfgroup.org/ftp/HDF/HDF_Current/src/hdf-${v}.tar.gz
tar -xf hdf-${v}.tar.gz
cd hdf-${v}
./configure --enable-shared --disable-fortran --prefix="$prefix"
make -j4
make install
)
rm -rf hdf-${v}
rm -f hdf-${v}.tar.gz

v=1.8.13
(set -xeu
wget -q ftp://ftp.unidata.ucar.edu/pub/netcdf/netcdf-4/hdf5-${v}.tar.gz
tar -xf hdf5-${v}.tar.gz && cd hdf5-${v}
./configure --enable-shared --enable-hl --prefix="$prefix"
make -j4
make install
)
rm -rf hdf5-${v}
rm -f hdf5-${v}.tar.gz

v=4.1.3
(set -xeu
wget -q http://www.unidata.ucar.edu/downloads/netcdf/ftp/netcdf-${v}.tar.gz
tar -xf netcdf-${v}.tar.gz && cd netcdf-${v}
./configure --enable-netcdf-4 --enable-shared --enable-dap --prefix="$prefix"
make -j4
make install
)
rm -rf netcdf-${v}
rm -f netcdf-${v}.tar.gz

v=2.9.8
(set -xeu
wget -q ftp://xmlsoft.org/libxml2/libxml2-${v}.tar.gz
tar -xf libxml2-${v}.tar.gz
cd libxml2-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf libxml2-${v}
rm -f libxml2-${v}.tar.gz

v=0.13.1
(set -xeu
wget -q https://s3.amazonaws.com/json-c_releases/releases/json-c-${v}.tar.gz
tar -xf json-c-${v}.tar.gz
cd json-c-${v}
./configure --prefix="$prefix"
make -j4
make install
)
rm -rf json-c-${v}
rm -f json-c-${v}.tar.gz

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
echo “Finished installing the dependencies. Now, run the 'build_gsky.sh'”
