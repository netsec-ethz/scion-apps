# web-gateway
web-gateway is a SCION web server that proxies web content from the TCP/IP web
to the SCION web.

## Installation

Build the `scion-web-gateway` binary by running `make scion-web-gateway` (see
[Build](../README.md#build) in the main README).

## Usage

This requires a running SCION endhost stack, i.e. a running SCION dispatcher
and SCION daemon. Please refer to '[Running](../../README.md#Running)' in this
repository's main README and the [SCIONLab tutorials](https://docs.scionlab.org) to get started.

Start the gateway, telling it which TCP/IP hosts it should be mirroring:
```
bin/scion-ip-gateway scionlab.org www.scionlab.org www.scion-architecture.net
```

While this server is running, you can access these websites over SCION, either
using the [`scion-bat`](../bat/README.md) command line tool, or using the
[`scion-skip`](../skip/README.md) browser integration.

First, add a hostname entry for the mirrored hosts, pointing it to the SCION
address that the gateway is running, e.g. add the following line to `/etc/scion/hosts`:
```
17-ffaa:1:a,127.0.0.1 scionlab.org www.scionlab.org www.scion-architecture.net
```

Then simply access it:
```
bin/scion-bat http://www.scion-architecture.net/pages/publications/
bin/scion-bat https://www.scion-architecture.net/pages/publications/
```
