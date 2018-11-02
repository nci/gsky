#!/bin/bash
#####################################################################
# build_all.sh
# Installs all dependencies for GSKY and build the GSKY environment on a VM
# Created on: 23 October, 2018; Arapaut V. Sivaprasad.
# Last Revision: 29 Oct, 2018; Arapaut V. Sivaprasad.
# Adapted from 'build_deps.sh' and 'build_gsky.sh' by Jian Edison Guo.
#####################################################################
# Usage: 
#	1. Place this script in your login dir or any convenient subdir. e.g. cp build_all.sh ~
#	2. chmod 755 build_all.sh
#	3. Execute the script. e.g. 'sudo ./build_all.sh'. Will take 45 to 60 min.
#	4. Watch for error exit, if any. If none, should say "Completed ALL steps. Exitting!"
# Note: This script requires CentOS 7 or later operating system.
#####################################################################

# Define which programs to install. Blank out the not-needed ones. e.g. dep1=""
git="Yes"; devtools="Yes"; dep1="Yes"; dep2="Yes"; dep3="Yes"; dep4="Yes"
dep5="Yes"; dep6="Yes"; dep7="Yes"; dep8="Yes"; dep9="Yes"; dep10="Yes"
dep11="Yes"; dep12="Yes"; dep13="Yes"; dep14="Yes"; dep15="Yes"; dep16="Yes"

home=`pwd`
if [ $git ]
then
	repo=asivapra # production repo: nci; Dev repo: asivapra
	echo "Cloning the GIT repositry: $repo"
	rm -rf gsky
	# Git clone the required files to your own workspace. These will be owned by root
	git clone https://github.com/${repo}/gsky.git
fi

# Installation happens in the required dirs accessible only by root. 
# The files created in the 'install' dir will be deleted on success.
mkdir -p gsky/install

# Change ownership to the user, so that the files can be edited, if required.
chown -R $SUDO_USER gsky

cd gsky/install

if [ $devtools ]
then
	echo "---> Installing the Development Tools and other dependencies."
	# Install the development tools under CentOS
	# 'yes|' means no confirmation before proceeding with removal and installation
	yes|yum groupremove "Development Tools"
	yes|yum groupinstall "Development Tools"
	yes|yum remove wget
	yes|yum install wget
	yes|yum remove cmake
	yes|yum install cmake
	yes|yum remove python-devel
	yes|yum install python-devel
	yes|yum install readline-devel
fi
#------------------------------------------------------------------------------------------------------------------
# Install GSKY-specific dependencies
#------------------------------------------------------------------------------------------------------------------
prefix=${PREFIX:-/usr}
if [ $dep1 ]
then
	echo "1. Installing: Independent JPEG Group's free JPEG software"
	v=9c
	(
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep2 ]
then
	echo "2.	 Installing: OPENJPEG Library and Applications"
	v=2.3.0
	(
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep3 ]
then
	echo "3.	 Installing: GEOS - Geometry Engine, Open Source"
	v=3.6.2
	(
		prefix=${PREFIX:-/usr}
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep4 ]
then
	echo "4.	 Installing: Cartographic Projection Procedures for the UNIX Environment"
	v=5.1.0
	vd=1.7
	(
		set -xeu
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
	rm -f proj-${v}.tar.gz*
	rm -f proj-datumgrid-${vd}.zip
	echo "Finished installing: Cartographic Projection Procedures for the UNIX Environment"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep5 ]
then
	echo "5.	 Installing: Zlib Data Compression Library"
	v=1.2.8
	(
		set -xeu
		wget -q ftp://ftp.unidata.ucar.edu/pub/netcdf/netcdf-4/zlib-${v}.tar.gz
		tar -xf zlib-${v}.tar.gz && cd zlib-${v}
		./configure --prefix="$prefix"
		make -j4
		make install
	)
	rm -rf zlib-${v}
	rm -f zlib-${v}.tar.gz
	echo "Finished installing: Zlib Data Compression Library"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep6 ]
then
	echo "6.	 Installing: HDF4 "
	v=4.2.13
	(
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep7 ]
then
	echo "7.	 Installing: HDF5 "
	v=1.8.13
	(
		set -xeu
		wget -q ftp://ftp.unidata.ucar.edu/pub/netcdf/netcdf-4/hdf5-${v}.tar.gz
		tar -xf hdf5-${v}.tar.gz && cd hdf5-${v}
		./configure --enable-shared --enable-hl --prefix="$prefix"
		make -j4
		make install
	)
	rm -rf hdf5-${v}
	rm -f hdf5-${v}.tar.gz
	echo "Finished installing: HDF5"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep8 ]
then
	echo "8.	 Installing: NetCDF "
	v=4.1.3
	(
		set -xeu
		wget -q http://www.unidata.ucar.edu/downloads/netcdf/ftp/netcdf-${v}.tar.gz
		tar -xf netcdf-${v}.tar.gz && cd netcdf-${v}
		./configure --enable-netcdf-4 --enable-shared --enable-dap --prefix="$prefix"
		make -j4
		make install
	)
	rm -rf netcdf-${v}
	rm -f netcdf-${v}.tar.gz
	echo "Finished installing: NetCDF"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep9 ]
then
	echo "9.  Installing: XML C parser "
	v=2.9.8
	(
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep10 ]
then
	echo "10.  Installing: JSON-C - A JSON implementation in C"
	v=0.13.1
	(
		set -xeu
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep11 ]
then
	echo "11.  Installing: GDAL with OpenJPEG support"
	v=2.3.1
	(
		set -xeu
		wget -q http://download.osgeo.org/gdal/${v}/gdal-${v}.tar.gz
		tar -xf gdal-${v}.tar.gz
		
		# Location of the openjpeg libraries.
		include=/usr/include
		
		# Find out where the openjpeg libraries are. 
		# This step is for safety, as sometimes the libraries are in /include 
		res=`find /usr/. -name libopenjp2.pc`
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
	echo "Finished: GDAL with OpenJPEG support"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep12 ]
then
	# Install PostGreSQL
	echo "12. Installing: PostGreSQL"
	v=11.0
	(
		set -xeu
		wget -q https://ftp.postgresql.org/pub/source/v${v}/postgresql-${v}.tar.gz
		tar -xf postgresql-${v}.tar.gz
		cd postgresql-${v}
		./configure
		make
		make install
	)
	rm -rf postgresql-${v}
	rm -f postgresql-${v}.tar.gz*
	
	# Check and create the user 'postgres'
	s1=`id -u postgres`
	echo $s1
	if [ $s1 ]
	then
		echo "User, 'postgres', exists. Nothing to do!"
	else
		echo "Creating the user!"
		adduser postgres
	fi
	mkdir -p /var/lib/pgsql
	mkdir -p /usr/local/pgsql/data
	chown postgres /usr/local/pgsql/data
	su - postgres -c '/usr/local/pgsql/bin/initdb -D /usr/local/pgsql/data; /usr/local/pgsql/bin/postgres -D /usr/local/pgsql/data >/tmp/logfile 2>&1 &'
	echo "Finished PostgreSQL Installation"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep13 ]
then
	# Install PostGIS. This should be done after installing other deps
	echo "13. Installing: PostGIS"
	v=2.5.0
	(
		set -xeu
		wget -q https://download.osgeo.org/postgis/source/postgis-${v}.tar.gz
		tar xf postgis-${v}.tar.gz
		cd postgis-${v}
		
		# The default loader/Makefile tries to compile 'pgsql2shp' and 'shp2pgsql', and crashes.
		# Therefore, mask it out in Makefile.in to prevent them being compiled. 
		# These executables are then copied/downloaded from elsewhere.
		#yes|cp ../postgis-2.5.0_Makefile.in loader/Makefile.in
		./configure --with-pgconfig=/usr/local/pgsql/bin/pg_config
		make
		
		# The folowing executables are required for 'make install'. Currently they are being copied from another location.
		#yes|cp ../pgsql2shp loader
		#yes|cp ../shp2pgsql loader
		make install
	)
	rm -f postgis-${v}.tar.gz*
	rm -rf postgis-${v}
	echo "Finished Installing: PostGIS"
	echo "**** Finished installing ALL the dependencies. ****"
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep14 ]
then
	echo "14. Install GO"
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
	(
		set -exu
		cd $GOPATH/src/github.com/${repo}/gsky
		./configure
		make all
	)	
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep15 ]
then
	echo "Copy all files to final locations"
	prefix=/local/gsky	
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
fi
#------------------------------------------------------------------------------------------------------------------
if [ $dep16 ]
then
	echo "Create the config.json and start the server"
	prefix=/local/gsky

	echo "Put a soft link to find the /usr/local/share/gsky"
	if [ ! -L /usr/local/share/gsky ] 
	then
		ln -s /local/gsky/share/gsky /usr/local/share/gsky
	fi
	
	if [ ! -f $prefix/share/gsky/config.json ] 
	then
		echo "Creating a sample config.json"	
		input=$home/gsky/install/config.json
		ip=`curl ifconfig.me`
		while IFS= read -r var
		do
		  line=${var/OWS_IP_ADDRESS/$ip}
		  echo "$line" >> $prefix/share/gsky/config.json
		done < "$input"
	fi	
	echo "Create a soft link to the config.json from /usr/local/etc"
	if [ ! -L /usr/local/etc/config.json ] 
	then
		ln -s $prefix/share/gsky/config.json /usr/local/etc/config.json
	fi
	echo "Start the OWS server"
	export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib
	/local/gsky/share/gsky/gsky -p 80&
fi
#------------------------------------------------------------------------------------------------------------------
echo "Completed ALL steps. Exiting!"
exit
