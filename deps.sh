#!/bin/bash
set -e

# version less or equal. E.g. verleq 1.9 2.0.8  == true (1.9 <= 2.0.8)
verleq() {
    [ ! -z "$1" ] && [ ! -z "$2" ] && [ "$1" = `echo -e "$1\n$2" | sort -V | head -n1` ]
}

V=$(go version | sed 's/^go version go\(.*\) .*$/\1/')
if ! verleq 1.11.0 "$V"; then
    echo "Go version 1.11 or newer required"
    exit 1
fi
sudo apt-get install -y libpam0g-dev
command -v govendor >/dev/null || go get github.com/kardianos/govendor
govendor sync -v
