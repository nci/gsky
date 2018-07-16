#!/bin/bash
set -xeu

apt-get update && apt-get install -y --no-install-recommends python2.7

# npm has permission issues with docker root user
# so we create a dummy user to bypass these issues
useradd dummy_user
mkdir -p /home/dummy_user
chown dummy_user:dummy_user /home/dummy_user

su dummy_user <<'EOF'
set -xeu
cd /home/dummy_user

vnode=8.11.3
wget -q https://nodejs.org/dist/v${vnode}/node-v${vnode}-linux-x64.tar.xz
tar -xf node-v${vnode}-linux-x64.tar.xz
export PATH="`pwd`/node-v${vnode}-linux-x64/bin:$PATH"

git clone https://github.com/TerriaJS/TerriaMap.git

cd TerriaMap
npm install
npm run gulp release
EOF

chown -R root:root /home/dummy_user/TerriaMap/wwwroot
mkdir -p /gsky/share/gsky/static
cp -r /home/dummy_user/TerriaMap/wwwroot /gsky/share/gsky/static/terria

#userdel also performs rm -rf /home/dummy_user
userdel -r dummy_user
