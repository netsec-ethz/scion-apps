# scion-apps

This repo contains demo applications using the SCION protocol.

The applications are written in Go, with some supporting code in Python. A SCION Internet connection (for instance via SCIONLab) is required to run these applications.

More information on [SCION](https://www.scion-architecture.net/), and [tutorials on how to set up SCION and SCIONLab](https://netsec-ethz.github.io/scion-tutorials/).

### Installation:
Download and install our Debian packages:
```shell
sudo apt-get install apt-transport-https
echo "deb [trusted=yes] https://packages.netsec.inf.ethz.ch/debian all main" | sudo tee /etc/apt/sources.list.d/scionlab.list
sudo apt-get update
sudo apt install scion-apps-*
```

### Build:

Building the `SSH` tool requires `libpam0g-dev`:

```shell
sudo apt-get install -y libpam0g-dev
```

Run `make` to build all projects.
Run `make install` to build all projects and install into `$GOPATH/bin`


## bat

bat is a CLI cURL-like tool for testing, debugging, and generally interacting with HTTP servers over SCION/QUIC. Documentation of the code is available in the [README.md](https://github.com/netsec-ethz/scion-apps/blob/master/bat/README.md)


## camerapp

Camerapp contains image fetcher and server applications, using the SCION network. Documentation of the code is available in the [README.md](https://github.com/netsec-ethz/scion-apps/blob/master/camerapp/README.md)

Installation and usage information is available on the [SCION Tutorials web page for camerapp](https://netsec-ethz.github.io/scion-tutorials/sample_projects/access_camera/).


## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

Installation and usage information is available on the [SCION Tutorials web page for sensorapp](https://netsec-ethz.github.io/scion-tutorials/sample_projects/fetch_sensor_readings/).


## bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network.

Documentation of the code and protocol are described in the [bwtester README](https://github.com/netsec-ethz/scion-apps/blob/master/bwtester/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bwtester](https://netsec-ethz.github.io/scion-tutorials/sample_projects/bwtester/).


## roughtime

Implementation of server and client applications, running the "roughtime" protocol over the SCION network. Roughtime is a project that aims to provide secure time synchronisation. More information on the project can be found on the [original repository](https://roughtime.googlesource.com/roughtime)


## webapp

Webapp is a Go application that will serve up a static web portal to make it easy to experiment with SCIONLab test apps on a virtual machine.

Installation and usage information is available on the [SCION Tutorials web page for webapp](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp/).


## helloworld

A simple demo application using SCION that sends one packet.
