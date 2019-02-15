# bat

![](images/bat_output.png "sample output of bat application")

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

In contrast to the original tool, we require a local SCION address.
If you're running bat in a VM, your local address can be inferred:

```
bat GET https://server/api/download
```

In case this fails or you are running a different setup, provide the local address using the ```-l``` flag:

```
bat -l ISD-AS,[IP]:port GET https://server/api/download
```

The scheme defaults to HTTPS. The method defaults to GET in case there is no data to be sent and to POST otherwise.

Hostnames are resolved by parsing the `/etc/hosts` file. Known hosts can be added by adding lines like this:

```
# The following lines are SCION hosts
17-ffaa:1:10,[10.0.8.100]	server1
18-ffaa:0:11,[10.0.8.120]	server2
```

### Examples

| Request                                             | Explanation                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------ |
| bat server:8080/api/download                        | HTTPS GET request to server:8080/download                          |
| bat -b server:8080/api/download                     | Run a benchmark against server:8080/download                       |
| bat server:8080/api/upload foo=bar                  | HTTPS POST request with JSON encoded data<br>to server:8080/upload |
| bat -f server:8080/api/upload foo=bar               | HTTPS POST request with URL encoded data<br>to server:8080/upload  |
| bat -body "Hello World" POST server:8080/api/upload | HTTPS POST request with raw data<br>to server:8080/upload          |