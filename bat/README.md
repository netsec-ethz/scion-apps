# bat

![](images/bat_output.png "sample output of bat application")

Go implemented CLI cURL-like tool for humans. Bat can be used for testing, debugging, and generally interacting with HTTP servers.

This repository is a fork of [astaxie/bat](https://github.com/astaxie/bat) making it available for SCION/QUIC.
Refer to the original repository for general usage.

### Usage

```
bat <method> <url>
```

The scheme defaults to HTTPS -- HTTP is not supported. The method defaults to GET in case there is no data to be sent and to POST otherwise.

URLs can use SCION addresses or hostnames. Hostnames are resolved by scanning the `/etc/hosts` file or by a RAINS lookup (if configured) -- see the toplevel README.

### Examples

| Request                                             | Explanation                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------ |
| bat server:8080/api/download                        | HTTPS GET request to server:8080/download                          |
| bat 17-ffaa:1:10,[10.0.8.100]:8080/api/download     | HTTPS GET request to 17-ffaa:1:10,[10.0.8.100]:8080/download       |
| bat -b server:8080/api/download                     | Run a benchmark against server:8080/download                       |
| bat server:8080/api/upload foo=bar                  | HTTPS POST request with JSON encoded data<br>to server:8080/upload |
| bat -f server:8080/api/upload foo=bar               | HTTPS POST request with URL encoded data<br>to server:8080/upload  |
| bat -body "Hello World" POST server:8080/api/upload | HTTPS POST request with raw data<br>to server:8080/upload          |
