set -xeu
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

go get github.com/nci/gsky
rm -rf $GOPATH/src/github.com/nci/gsky
git clone https://github.com/nci/gsky.git $GOPATH/src/github.com/nci/gsky

(set -exu
cd $GOPATH/src/github.com/nci/gsky
./configure
make all
)
rm -rf $prefix/share
mkdir -p $prefix/share/gsky
mkdir -p $prefix/share/mas
yes|cp -f $GOPATH/src/github.com/nci/gsky/concurrent $prefix/bin/concurrent
yes|cp -f $GOPATH/bin/api $prefix/bin/api
yes|cp -f $GOPATH/bin/gsky $prefix/share/gsky/gsky
yes|cp -f $GOPATH/bin/grpc-server $prefix/share/gsky/grpc_server
yes|cp -f $GOPATH/bin/gdal-process $prefix/share/gsky/gsky-gdal-process
yes|cp -f $GOPATH/bin/crawl $prefix/share/gsky/gsky-crawl
yes|cp -f $GOPATH/src/github.com/nci/gsky/crawl/crawl_pipeline.sh $prefix/share/gsky/crawl_pipeline.sh
yes|cp -f $GOPATH/src/github.com/nci/gsky/mas/db/* $prefix/share/mas/

yes|cp -rf $GOPATH/src/github.com/nci/gsky/*.png $prefix/share/gsky/
yes|cp -rf $GOPATH/src/github.com/nci/gsky/templates $prefix/share/gsky/
yes|cp -rf $GOPATH/src/github.com/nci/gsky/static $prefix/share/gsky/

rm -rf /local/gsky_temp
mkdir -p /local/gsky_temp
chown -R nobody:nobody /local/gsky_temp
echo "Finished installing the GSKY server."

