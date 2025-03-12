# scion-apps

This repo contains demo applications using the SCION protocol.

The applications are written in Go, with some supporting code in Python. A SCION Internet connection (for instance via SCIONLab) is required to run these applications.

More information on [SCION](https://www.scion-architecture.net/), and [tutorials on how to set up SCION and SCIONLab](https://docs.scionlab.org/).

### Installation:
Download and install our Debian packages:
```shell
sudo apt-get install apt-transport-https
echo "deb [trusted=yes] https://packages.netsec.inf.ethz.ch/debian all main" | sudo tee /etc/apt/sources.list.d/scionlab.list
sudo apt-get update
sudo apt install scion-apps-*
```

### Build:

1. Install Go. We currently support Go versions 1.19 and 1.20.

    See e.g. https://github.com/golang/go/wiki/Ubuntu

1. Install dependencies

    Building the SSH tool requires `libpam0g-dev`:

    ```shell
    sudo apt-get install -y libpam0g-dev
    ```

1. Clone this repository

   ```shell
   git clone https://github.com/netsec-ethz/scion-apps.git
   ```
   _Note: because this is using go modules, there is no need to put this under `$GOPATH`_

1. Install `golangci-lint`, which is used to run format checks and various linter steps

    ```shell
    make setup_lint
    ```

1. Run `make` to build all projects (and run linters). Run `make -j` to use multiple jobs to build.

1. Run `make test` to run the linters and unit tests.

1. Run `make install` to build all projects and install into `$GOPATH/bin`.


### Running:

All of these applications require a running SCION endhost stack, i.e. a running SCION daemon and depending on the environment a running SCION dispatcher. The SCION dispatcher is needed if you plan to use your SCION application on a port outside the defined `dispatched_ports` range or with legacy versions of the BR (previous versions to 0.12.0 [release](https://github.com/scionproto/scion/releases)).
Please refer to the [SCIONLab tutorials](https://docs.scionlab.org) to get
started.


#### Environment

The SCION dispatcher listens for incoming packets on the unspecified IPv6 address ([::]).
You can modify this by changing the value on the `dispatcher.toml` configuration.

The SCION daemon is assumed to be at the default address, but this can be
overridden using an environment variable:

		SCION_DAEMON_ADDRESS: 127.0.0.1:30255

This is convenient for the normal use case of running the endhost stack for a
single SCION AS. When running multiple local ASes, e.g. during development, the
address of the SCION daemon corresponding to the desired AS needs to be
specified in the `SCION_DAEMON_ADDRESS` environment variable.
In this case, the different daemon addresses can be found in their corresponding
`sd.toml` configuration files in the `gen/ASx` directory, or summarized in the
file `gen/sciond_addresses.json`.


#### Hostnames
Hostnames are resolved by scanning `/etc/hosts`, `/etc/scion/hosts` and by a RAINS lookup.

Hosts can be added to `/etc/hosts`, or `/etc/scion/hosts` by adding lines like this:

```
# The following lines are SCION hosts
17-ffaa:1:10,[10.0.8.100] server1
18-ffaa:0:11,[10.0.8.120] server2
```

The RAINS resolver address can be configured in `/etc/scion/rains.cfg`.
This configuration file needs to contain the SCION address of the RAINS
resolver, in the form `<ISD>-<AS>,[<IP>]`.


## _examples

The directory _examples contains examples for the usage of the SCION libraries.

* [_examples/helloworld](_examples/helloworld/README.md):
  A minimal "hello, world" application using UDP over SCION.
* [_examples/helloquic](_examples/helloquic/README.md):
  Example for the use of QUIC over SCION.
* [_examples/sgrpc](_examples/sgrpc/README.md):
  Example for using gRPC over SCION/QUIC with the PAN library.
* [_examples/shttp](_examples/shttp/README.md):
  Examples for using HTTP over SCION/QUIC, examples for servers, proxies, and clients.

## bat

bat is a cURL-like tool for testing, debugging, and generally interacting with HTTP servers over SCION/QUIC. Documentation of the code is available in the [bat README](bat/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bat](https://docs.scionlab.org/content/apps/bat.html).


## bwtester

The bandwidth testing application bwtester enables a variety of bandwidth tests on the SCION network. Documentation of the code and protocol are described in the [bwtester README](bwtester/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bwtester](https://docs.scionlab.org/content/apps/bwtester.html).


## netcat

netcat contains a SCION port of the netcat application. See the [netcat README](netcat/README.md) for more information.


## pkg

Pkg contains underlaying library code for scion-apps.

- pan: Policy-based, path aware networking library, wrapper for the SCION core libraries
- shttp: glue library to use net/http libraries for HTTP over SCION
- shttp3: glue library to use quic-go/http3 for HTTP/3 over SCION
- quicutil: contains utilities for working with QUIC
- integration: a simple framework to support intergration testing for the demo applications in this repository


## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

Installation and usage information is available on the [SCION Tutorials web page for sensorapp](https://docs.scionlab.org/content/apps/fetch_sensor_readings.html).

## skip

skip is a very simple local HTTP proxy server for very basic SCION browser support. See the [skip README](skip/README.md) for more information.

## ssh

Directory ssh contains a SSH client and server running over SCION network.

More documentation is available in the [ssh README](ssh/README.md).


## web-gateway

web-gateway is a SCION web server that proxies web content from the TCP/IP web to the SCION web.

## webapp

Webapp is a Go application that will serve up a static web portal to make it easy to experiment with SCIONLab test apps on a virtual machine.

Installation and usage information is available on the [SCION Tutorials web page for webapp](https://docs.scionlab.org/content/apps/as_visualization/webapp.html) and in the [webapp README](webapp/README.md).
