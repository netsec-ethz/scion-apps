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

If you experience problems with the `govendor` commands above:
* Remove the contents and leave only the `vendor.json` file in the `vendor` folders in `scion-apps` and `bat` itself.
* Go to the `scion-apps` directory and run `govendor sync`. You can now follow the instructions above again.

### Usage

In contrast to the original tool, we require a remote SCION address and a URL path instead of a full URL.
For example:

```
bat -r ISD-AS,[IP]:port GET /api/download
```

If you're running bat in a VM, your local address can be inferred. In case this fails or you are running a different setup, provide the local address using the ```-l``` flag:

```
bat -l ISD-AS,[IP]:port -r ISD-AS,[IP]:port GET /api/download
```

The HTTP method defaults to GET in case there is no data to be sent and to POST otherwise.

### Examples

```
bat -r ISD-AS,[IP]:port /api/download

bat -r ISD-AS,[IP]:port /api/upload foo=bar

bat -r ISD-AS,[IP]:port -f /api/upload foo=bar

bat -r ISD-AS,[IP]:port -body "Hello World" POST /api/upload
```