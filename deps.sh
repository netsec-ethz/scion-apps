#!/bin/bash
set -x

printf "\n### Getting govendor ###\n"
if ! govendor -version 2>&1 | grep v1.0.8  >& /dev/null; then
    GOVENDORDIR="$HOME/go/src/github.com/kardianos/govendor"
    [ -d "$GOVENDORDIR" ] || git clone "https://github.com/kardianos/govendor" "$GOVENDORDIR"
    cd "$GOVENDORDIR" && git checkout fbbc78e8d1b533dfcf81c2a4be2cec2617a926f7 && go install -v
fi

printf "\n### Getting vendor libraries\n"
govendor sync -v

printf "\n### Getting dependencies\n"
sudo apt-get install -y capnproto libpam0g-dev
if ! command -v capnpc-go >&/dev/null; then
    cd vendor/zombiezen.com/go/capnproto2/capnpc-go && go install -v
fi

printf "\n### Compiling capnp ###\n"
cp vendor/zombiezen.com/go/capnproto2/std/go.capnp vendor/github.com/scionproto/scion/proto/go.capnp
cd vendor/github.com/scionproto/scion/go/proto && make
