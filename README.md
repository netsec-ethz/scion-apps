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

1. Install go 1.13

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
    
1. Run `make` build all projects (and run linters). Run `make -j` to use multiple jobs to build.
   
1. Run `make test` to 

1. Run `make install` to build all projects and install into `$GOPATH/bin`


### Running:

All of these applications require a running SCION endhost stack, i.e. a running
SCION dispatcher and SCION daemon.
Please refer to the [SCIONLab tutorials](https://docs.scionlab.org) to get
started.

#### Environment

The dispatcher and sciond sockets are assumed to be at default locations, but
this can be overriden using environment variables:

		SCION_DISPATCHER_SOCKET: /run/shm/dispatcher/default.sock
		SCION_DEAMON_SOCKET: /run/shm/sciond/default.sock

This is convenient for the normal use case of running a the endhost stack for a
single SCION AS.
When running multiple local ASes, e.g. during development, the path to the
sciond corresponding to the desired AS needs to be specified in the
`SCION_DEAMON_SOCKET` environment variable.


#### Hostnames
Hostnames are resolved by scanning the `/etc/hosts` file and by a RAINS lookup.

Hosts can be added to `/etc/hosts` by adding lines like this:

```
# The following lines are SCION hosts
17-ffaa:1:10,[10.0.8.100]	server1
18-ffaa:0:11,[10.0.8.120]	server2
```

The RAINS resolver address can be configured in `/etc/scion/rains.cfg`.
This configuration file needs to contain the SCION address of the RAINS
resolver, in the form `<ISD>-<AS>,[<IP>]`.


## bat

bat is a CLI cURL-like tool for testing, debugging, and generally interacting with HTTP servers over SCION/QUIC. Documentation of the code is available in the [README.md](bat/README.md)

Installation and usage information is available on the [SCION Tutorials web page for bat](https://docs.scionlab.org/content/apps/bat.html).

## camerapp

Camerapp contains image fetcher and server applications, using the SCION network. Documentation of the code is available in the [README.md](https://github.com/netsec-ethz/scion-apps/blob/master/camerapp/README.md)

Installation and usage information is available on the [SCION Tutorials web page for camerapp](https://docs.scionlab.org/content/apps/access_camera.html).


## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

Installation and usage information is available on the [SCION Tutorials web page for sensorapp](https://docs.scionlab.org/content/apps/fetch_sensor_readings.html).


## bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network.

Documentation of the code and protocol are described in the [bwtester README](bwtester/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bwtester](https://docs.scionlab.org/content/apps/bwtester.html).


## webapp

Webapp is a Go application that will serve up a static web portal to make it easy to experiment with SCIONLab test apps on a virtual machine.

Installation and usage information is available on the [SCION Tutorials web page for webapp](https://docs.scionlab.org/content/apps/as_visualization/webapp.html).
