#!/bin/bash
set -xeu
DEFAULT_GSKY_REPO='https://github.com/nci/gsky.git'
gsky_repo="${1:-$DEFAULT_GSKY_REPO}"

C_INCLUDE_PATH=$(nc-config --includedir)
export C_INCLUDE_PATH

wget -q -O go.tar.gz https://dl.google.com/go/go1.16.3.linux-amd64.tar.gz
rm -rf go && tar -xf go.tar.gz && rm -f go.tar.gz

export GOROOT=/go
export GOPATH=/gsky/gopath
export PATH="$PATH:$GOROOT/bin"

rm -rf $GOPATH && mkdir $GOPATH
git clone "$gsky_repo" $GOPATH/src/github.com/nci/gsky

(set -xeu
cd $GOPATH/src/github.com/nci/gsky

mkdir -p /gsky
./configure --prefix=/gsky --bindir=/gsky/bin --sbindir=/gsky/bin --libexecdir=/gsky/bin
make all
make install
)
