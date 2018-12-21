#!/bin/bash
printf "\n### Getting dependencies ###\n"
govendor sync -v
printf "\n### Compiling capnp ###\n"
cp vendor/zombiezen.com/go/capnproto2/std/go.capnp vendor/github.com/scionproto/scion/proto/go.capnp
cd vendor/github.com/scionproto/scion/go/proto && make