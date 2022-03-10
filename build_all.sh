#!/usr/bin/env bash

CURDIR=`/bin/pwd`
BASEDIR=$(dirname $0)
ABSPATH=$(readlink -f $0)
ABSDIR=$(dirname $ABSPATH)


unset GOPATH

version=`cat ./config/version.go  | grep -i version |cut -d\" -f 2`


cd $CURDIR
bash $ABSDIR/build_package.sh "./cmd/derod"
bash $ABSDIR/build_package.sh "./cmd/explorer"
bash $ABSDIR/build_package.sh "./cmd/dero-wallet-cli"
bash $ABSDIR/build_package.sh "./cmd/dero-miner"
bash $ABSDIR/build_package.sh "./cmd/simulator"
#bash $ABSDIR/build_package.sh "./cmd/rpc_examples/pong_server"


for d in build/*; do cp Start.md "$d"; done
cd "${ABSDIR}/build"



#windows users require zip files
zip -r dero_windows_amd64.zip dero_windows_amd64
zip -r dero_windows_amd64_$version.zip dero_windows_amd64


#macos needs universal fat binaries, so lets build them
mkdir -p dero_darwin_universal
go run github.com/randall77/makefat ./dero_darwin_universal/derod-darwin  ./dero_darwin_amd64/derod-darwin-amd64 ./dero_darwin_arm64/derod-darwin-arm64
go run github.com/randall77/makefat ./dero_darwin_universal/explorer-darwin  ./dero_darwin_amd64/explorer-darwin-amd64 ./dero_darwin_arm64/explorer-darwin-arm64
go run github.com/randall77/makefat ./dero_darwin_universal/dero-wallet-cli-darwin  ./dero_darwin_amd64/dero-wallet-cli-darwin-amd64 ./dero_darwin_arm64/dero-wallet-cli-darwin-arm64
go run github.com/randall77/makefat ./dero_darwin_universal/dero-miner-darwin  ./dero_darwin_amd64/dero-miner-darwin-amd64 ./dero_darwin_arm64/dero-miner-darwin-arm64
go run github.com/randall77/makefat ./dero_darwin_universal/simulator-darwin  ./dero_darwin_amd64/simulator-darwin-amd64 ./dero_darwin_arm64/simulator-darwin-arm64
#go run github.com/randall77/makefat ./dero_darwin_universal/pong_server-darwin  ./dero_darwin_amd64/pong_server-darwin-amd64 ./dero_darwin_arm64/pong_server-darwin-arm64

rm -rf dero_darwin_amd64
rm -rf dero_darwin_arm64

#all other platforms are okay with tar.gz
find . -mindepth 1 -type d -not -name '*windows*'   -exec tar -cvzf {}.tar.gz {} \;
find . -mindepth 1 -type d -not -name '*windows*'   -exec tar -cvzf {}_$version.tar.gz {} \;




cd $CURDIR
