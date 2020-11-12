# ScionFTP: FTP client on the SCION network

This project aims to show feasibility when implementing an exisiting data transmission protocol on the SCION network. Furthermore, to make use of the multi-path property of SCION, we added the GridFTP extension that allows to send traffic on multiple connections.

## Installation

1. Make sure to have a [SCION installation](https://netsec-ethz.github.io/scion-tutorials/) running, either locally or on the SCIONLab.
2. Download this repository with `go get github.com/elwin/scionFTP`
3. Install [server](server/scionftp_server) and [client](client/scionftp) or develop your own applications using the packages `server` and `client`