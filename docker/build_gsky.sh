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

gsky_src_root=/gsky/gsky_src
mkdir -p $gsky_src_root

git clone "$gsky_repo" $gsky_src_root/gsky

(set -xeu
cd $gsky_src_root/gsky

./configure --prefix=/gsky --bindir=/gsky/bin --sbindir=/gsky/bin --libexecdir=/gsky/bin
make all
make install
)
