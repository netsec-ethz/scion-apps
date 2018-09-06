# scionlab

This repo contains software for supporting SCIONLab.

The repo currently contains two applications: camerapp and sensorapp. Both applications are written in Go, with some supporting code in Python. A SCION Internet connection (for instance via SCIONLab) is required to run these applications.

More information on [SCION](https://www.scion-architecture.net/), and [tutorials on how to set up SCION and SCIONLab](https://netsec-ethz.github.io/scion-tutorials/).

***

## camerapp

Camerapp contains image fetcher and server applications, using the SCION network. Documentation on the code is available in the [README.md](https://github.com/perrig/scionlab/blob/master/camerapp/README.md)

Installation and usage information is available on the [SCION Tutorials web page for camerapp](https://netsec-ethz.github.io/scion-tutorials/sample_projects/access_camera/).

***

## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

Installation and usage information is available on the [SCION Tutorials web page for sensorapp](https://netsec-ethz.github.io/scion-tutorials/sample_projects/fetch_sensor_readings/).

***

## bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network.

Documentation of the code and protocol are described in the [bwtester README](https://github.com/perrig/scionlab/blob/master/bwtester/README.md).

Installation and usage information is available on the [SCION Tutorials web page for bwtester](https://netsec-ethz.github.io/scion-tutorials/sample_projects/bwtester/).
