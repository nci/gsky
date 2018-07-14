#!/bin/bash
set -xeu
DEFAULT_GSKY_REPO='https://github.com/nci/gsky.git'
gsky_repo="${1:-$DEFAULT_GSKY_REPO}"

export C_INCLUDE_PATH=$(nc-config --includedir)

wget -q -O go.tar.gz https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
tar -xf go.tar.gz

mkdir /gopath
export GOROOT=/go
export GOPATH=/gopath
export PATH="$PATH:$GOROOT/bin"

(set -xeu
go get github.com/nci/gsky
if [ "$gsky_repo" != "$DEFAULT_GSKY_REPO" ]
then
  rm -rf $GOPATH/src/github.com/nci/gsky
  git clone $gsky_repo $GOPATH/src/github.com/nci/gsky
fi

cd $GOPATH/src/github.com/nci/gsky

mkdir -p /gsky
./configure --prefix=/gsky --bindir=/gsky/bin --sbindir=/gsky/bin --libexecdir=/gsky/bin
make all
make install
)

rm -f go.tar.gz
rm -rf gopath
