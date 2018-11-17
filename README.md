# bat

Go implemented CLI cURL-like tool for humans. Bat can be used for testing, debugging, and generally interacting with HTTP servers.

This repository is a fork of [astaxie/bat](https://github.com/astaxie/bat) making it available for SCION/QUIC.
Refer to the original repository for general usage.

### Installation

SCION infrastructure must be installed and running on your machine. Instructions on how to set this up can be found [here](https://github.com/netsec-ethz/netsec-scion).
Clone the repository and install using:

```
govendor sync
govendor add +e
go install
```

### Usage

In contrast to the original tool, we require a remote SCION address and a URL path instead of a full URL.
For example:

```
bat -r ISD/AS,[IP]:port GET /api/download
```

bat tries to infer your local SCION address for installations in VMs. In case this fails, provide the local address using the ```-l``` flag:

```
bat -l ISD/AS,[IP]:port -r ISD/AS,[IP]:port GET /api/download
```

As the original, the HTTP method defaults to GET in case there is no data to be sent and to POST otherwise.

### Examples

```
bat -r ISD/AS,[IP]:port /api/download

bat -r ISD/AS,[IP]:port /api/upload foo=bar

bat -r ISD/AS,[IP]:port -f /api/upload foo=bar

bat -r ISD/AS,[IP]:port -body "Hello World" POST /api/upload
```