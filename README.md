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

1. Install go 1.16

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

All of these applications require a running SCION endhost stack, i.e. a running
SCION dispatcher and SCION daemon.
Please refer to the [SCIONLab tutorials](https://docs.scionlab.org) to get
started.


#### Environment

The dispatcher and sciond sockets are assumed to be at default locations, but
this can be overridden using environment variables:

		SCION_DISPATCHER_SOCKET: /run/shm/dispatcher/default.sock
		SCION_DAEMON_ADDRESS: 127.0.0.1:30255

This is convenient for the normal use case of running the endhost stack for a
single SCION AS.
When running multiple local ASes, e.g. during development, the address of the
sciond corresponding to the desired AS needs to be specified in the
`SCION_DAEMON_ADDRESS` environment variable.
In this case, the different sciond addresses can be found in their
corresponding `sd.toml` configuration files in the `gen/ASx`
directory, or summarized in the file `gen/sciond_addresses.json`.


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

The directory _examples contains a minimal "hello, world" application using SCION that sends one packet from a client to a server,
as well as a simple "hello DRKey" application, showing how to use DRKey.
The directory also contains small example programs that show how HTTP can be used over SCION/QUIC for servers, proxies, and clients.

More documentation is available in the [helloworld README](_examples/helloworld/README.md), in the [hellodrkey README](_examples/hellodrkey/README.md)
and in the [shttp README](_examples/shttp/README.md).


## bat

bat is a cURL-like tool for testing, debugging, and generally interacting with HTTP servers over SCION/QUIC. Documentation of the code is available in the [bat README](bat/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bat](https://docs.scionlab.org/content/apps/bat.html).


## bwtester

The bandwidth testing application bwtester enables a variety of bandwidth tests on the SCION network. Documentation of the code and protocol are described in the [bwtester README](bwtester/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bwtester](https://docs.scionlab.org/content/apps/bwtester.html).


## burster

A tool that helps identifying problems with border routers dropping packets. It can also be used to add load to border routers. [README](burster/README.md).

## cbrtester

A tool intended to detect conditions on border routers, where packets are delayed more than the ordinary. [README](cbrtester/README.md).
## camerapp

Camerapp contains image fetcher and server applications, using the SCION network. Documentation of the code is available in the [camerapp README](camerapp/README.md).

Installation and usage information is available on the [SCION Tutorials web page for camerapp](https://docs.scionlab.org/content/apps/access_camera.html).


## netcat

netcat contains a SCION port of the netcat application. See the [netcat README](netcat/README.md) for more information.


## pkg

Pkg contains underlaying library code for scion-apps.

- appnet: simplified and functionally extended wrapper interfaces for the SCION core libraries
- appquic:  a simple interface to use QUIC over SCION
- shttp: a client/server implementation of HTTP/3 over SCION/QUIC
- integration: a simple framework to support intergration testing for the demo applications in this repository


## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

Installation and usage information is available on the [SCION Tutorials web page for sensorapp](https://docs.scionlab.org/content/apps/fetch_sensor_readings.html).

## skip

skip is a very simple local HTTP proxy server for very basic SCION browser support. See the [skip README](skip/README.md) for more information.

## ssh

Directory ssh contains a SSH client and server running over SCION network.

More documentation is available in the [ssh README](ssh/README.md).


## webapp

Webapp is a Go application that will serve up a static web portal to make it easy to experiment with SCIONLab test apps on a virtual machine.

Installation and usage information is available on the [SCION Tutorials web page for webapp](https://docs.scionlab.org/content/apps/as_visualization/webapp.html) and in the [webapp README](webapp/README.md).
