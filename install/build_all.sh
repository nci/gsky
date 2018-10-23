#!/bin/bash
#####################################################################
# build_all.sh
# Installs all dependencies for GSKY and build the GSKY environment on a VM
# 23 October, 2018; Dr. Arapaut V. Sivaprasad.
# Adapted from 'build_deps.sh' and 'build_gsky.sh'
#####################################################################
# **** NOTE ****: This shel script must be run as super user. 
# Usage: 'sudo ./build_all.sh' or 'sudo -i' and then './build_all.sh'.
#####################################################################

# Set the bash script to print the commands being executed and to exit on error
set -xeu

# Install the development tools under CentOS
# 'yes|' means no confirmation before proceeding with removal and installation
yes|yum groupremove "Development Tools"
yes|yum groupinstall "Development Tools"
yes|yum groupremove "PostgreSQL Database"
yes|yum groupinstall "PostgreSQL Database"
yes|yum remove postgis
yes|yum install postgis
yes|yum remove wget
yes|yum install wget
yes|yum remove cmake
yes|yum install cmake
yes|yum remove python-devel
yes|yum install python-devel
#------------------------------------------------------------------------------------------------------------------
# Install GSKY-specific dependencies
#------------------------------------------------------------------------------------------------------------------
# 1.	Independent JPEG Group's free JPEG software
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
echo "Finished installing: Independent JPEG Group's free JPEG software"
#------------------------------------------------------------------------------------------------------------------
#2.	OPENJPEG Library and Applications
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
echo "Finished installing: OPENJPEG Library and Applications"
#------------------------------------------------------------------------------------------------------------------
# 3.	GEOS - Geometry Engine, Open Source
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
echo "Finished installing: GEOS - Geometry Engine, Open Source"
#------------------------------------------------------------------------------------------------------------------
# 4.	Cartographic Projection Procedures for the UNIX Environment
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
echo "Finished installing: Cartographic Projection Procedures for the UNIX Environment"
#------------------------------------------------------------------------------------------------------------------
# 5.	Zlib Data Compression Library
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
echo "Finished installing: Zlib Data Compression Library"
#------------------------------------------------------------------------------------------------------------------
# 6.	HDF4 
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
echo "Finished installing: HDF4"
#------------------------------------------------------------------------------------------------------------------
#7.	HDF5 
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
echo "Finished installing: HDF5"
#------------------------------------------------------------------------------------------------------------------
# 8.	NetCDF 
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
echo "Finished installing: NetCDF"
#------------------------------------------------------------------------------------------------------------------
# 9.	XML C parser 
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
echo "Finished installing: XML C parser"
#------------------------------------------------------------------------------------------------------------------
# 10.	JSON-C - A JSON implementation in C
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
echo "Finished installing: JSON-C - A JSON implementation in C"
#------------------------------------------------------------------------------------------------------------------
# 11. Build GDL with OpenJPEG support
v=2.3.1
(set -xeu
wget -q http://download.osgeo.org/gdal/${v}/gdal-${v}.tar.gz
tar -xf gdal-${v}.tar.gz

# Find out where the openjpeg libraries are. They are usually in /usr/include or /include
include=/usr/include
res=`find /. -name libopenjp2.pc`
if [ $res ]
then
	p=${res/libopenjp2.pc/}
	p=${p/./}
	export PKG_CONFIG_PATH=$p
	q=`/usr/bin/pkg-config libopenjp2 --cflags`
	r=${q/\/openjpeg*/}
	include=${r/-I/}
fi

cd gdal-${v}
./configure --with-geos=yes --with-netcdf --with-openjpeg=$include
make -j4
make install
)
rm -rf gdal-${v}
rm -f gdal-${v}.tar.gz
echo "Finished installing: Build GDL with OpenJPEG support"
echo "**** Finished installing ALL the dependencies. ****"
#####################################################################
# Build the GSKY
#####################################################################
prefix=/local/gsky

mkdir -p $prefix

rm -rf $prefix/gopath
mkdir $prefix/gopath

rm -rf $prefix/bin
mkdir $prefix/bin

C_INCLUDE_PATH=$(/usr/bin/nc-config --includedir)
export C_INCLUDE_PATH

wget -q -O go.tar.gz https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
tar -xf go.tar.gz
rm -rf go.tar.gz

rm -rf $prefix/go
mv go $prefix/go

export GOROOT=$prefix/go
export GOPATH=$prefix/gopath
export PATH="$PATH:$GOROOT/bin"
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig

repo=nci
go get github.com/${repo}/gsky
rm -rf $GOPATH/src/github.com/${repo}/gsky
git clone https://github.com/${repo}/gsky.git $GOPATH/src/github.com/${repo}/gsky

(set -exu
cd $GOPATH/src/github.com/${repo}/gsky
./configure
make all
)	
rm -rf $prefix/share
mkdir -p $prefix/share/gsky
mkdir -p $prefix/share/mas
yes|cp -f $GOPATH/src/github.com/${repo}/gsky/concurrent $prefix/bin/concurrent
yes|cp -f $GOPATH/bin/api $prefix/bin/api
yes|cp -f $GOPATH/bin/gsky $prefix/share/gsky/gsky
yes|cp -f $GOPATH/bin/grpc-server $prefix/share/gsky/grpc_server
yes|cp -f $GOPATH/bin/gdal-process $prefix/share/gsky/gsky-gdal-process
yes|cp -f $GOPATH/bin/crawl $prefix/share/gsky/gsky-crawl
yes|cp -f $GOPATH/src/github.com/${repo}/gsky/crawl/crawl_pipeline.sh $prefix/share/gsky/crawl_pipeline.sh
yes|cp -f $GOPATH/src/github.com/${repo}/gsky/mas/db/* $prefix/share/mas/

yes|cp -rf $GOPATH/src/github.com/${repo}/gsky/*.png $prefix/share/gsky/
yes|cp -rf $GOPATH/src/github.com/${repo}/gsky/templates $prefix/share/gsky/
yes|cp -rf $GOPATH/src/github.com/${repo}/gsky/static $prefix/share/gsky/

rm -rf /local/gsky_temp
mkdir -p /local/gsky_temp
chown -R nobody:nobody /local/gsky_temp
echo "**** Finished installing the GSKY server. **** "
#------------------------------------------------------------------------------------------------------------------
exit
