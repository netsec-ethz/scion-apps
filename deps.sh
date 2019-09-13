#!/bin/bash
set -x

command -v govendor >/dev/null || go get github.com/kardianos/govendor
govendor sync -v
