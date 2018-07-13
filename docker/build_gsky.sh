#!/bin/bash
set -eu

export C_INCLUDE_PATH=$(nc-config --includedir)

wget -q -O go.tar.gz https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
tar -xf go.tar.gz

mkdir /gopath
export GOROOT=/go
export GOPATH=/gopath
export PATH="$PATH:$GOROOT/bin"

go get github.com/nci/gsky
rm -rf $GOPATH/src/github.com/nci/gsky
git clone https://github.com/edisonguo/gsky-1.git $GOPATH/src/github.com/nci/gsky
cd $GOPATH/src/github.com/nci/gsky
git checkout fix_crawler_build

mkdir /gsky
./configure --prefix=/gsky --bindir=/gsky/bin --sbindir=/gsky/bin --libexecdir=/gsky/bin
make all
make install
