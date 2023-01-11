# Example for gRPC over SCION/QUIC

This directory contains a small example program that shows how gRPC can be used over SCION/QUIC with the PAN library.
The example consists of an echo server and a client that sends a message to the service.

## Running

The example requires a running SCION endhost stack, i.e. a running SCION dispatcher and SCION daemon. Please refer to '[Running](../../README.md#Running)' in this repository's main README and the [SCIONLab tutorials](https://docs.scionlab.org) to get started.
See '[Environment](../../README.md#Environment)' on how to set the dispatcher and sciond environment variables when e.g. running multiple local ASes.

To test the server and client, run the SCION tiny test topology.

Open a shell and run the server in the AS `1-ff00:0:111`:
```bash
# Server in 1-ff00:0:111
SCION_DAEMON_ADDRESS="127.0.0.20:30255" \
go run server/main.go --server-address 127.0.0.1:5000
```

Open a shell and run the client in the AS `1-ff00:0:112` and send a message to the server:
```bash
# Client in 1-ff00:0:112
SCION_DAEMON_ADDRESS="127.0.0.28:30255" \
go run client/main.go --server-address "1-ff00:0:111,127.0.0.1:5000" --message "gRPC over SCION/QUIC"
```

## Protobuf
Tutorial: https://grpc.io/docs/languages/go/basics/

Requirements:
```bash
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

The compiler plugin protoc-gen-go will be installed in $GOBIN, defaulting to $GOPATH/bin. It must be in your $PATH for the protocol compiler protoc to find it.
```bash
export GO_PATH=~/go
export PATH=$PATH:/$GO_PATH/bin
```

The generation of the gRPC client and server interface is performed as follows:
```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/*.proto
```
