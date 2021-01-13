# Examples for HTTP over SCION/QUIC

This directory contains small example programs that show how HTTP can be used over SCION/QUIC for servers, proxies, and clients:

- fileserver: a server that serves the files from its working directory
- proxy: a proxy server that can translate between HTTP and HTTP-over-SCION
- server: a server with friendly greetings and other examples
- client: a client that talks to the example server

See also the package [shttp](../../pkg/shttp/README.md) for the underlaying library code.

## Build:

Clone the repository netsec-ethz/scion-apps and build the eaxample applications:

```
git clone https://github.com/netsec-ethz/scion-apps.git
cd scion-apps
make example-shttp-fileserver \
        example-shttp-proxy \
        example-shttp-server \
        example-shttp-client
```

## Running:

All examples require a running SCION endhost stack, i.e. a running SCION dispatcher and SCION daemon. Please refer to '[Running](../../README.md#Running)' in this repository's main README and the [SCIONLab tutorials](https://docs.scionlab.org) to get started.

### Generic file server example

Run `example-shttp-fileserver`:

```
bin/example-shttp-fileserver
```

See '[Environment](../../README.md#Environment)' on how to set the dispatcher and sciond environment variables in the server's AS.

Build `scion-bat` as a client for `example-shttp-fileserver`:

```
make scion-bat
```

See also the application '[bat](../../bat/README.md)' for more details on the cURL-like CLI tool `scion-bat`.

Access `example-shttp-fileserver` with `scion-bat`:

```
bin/scion-bat 17-ffaa:1:a,[127.0.0.1]:443/
```

Replace '17-ffaa:1:a' with your server's ISD and AS numbers and see '[Environment](../../README.md#Environment)' on how to set the dispatcher and sciond environment variables in the client's (or proxy's) AS.

Run `example-shttp-proxy` to provide `example-shttp-fileserver` functionality via HTTP:

```
bin/example-shttp-proxy --remote=17-ffaa:1:a,[127.0.0.1]:443 --local=0.0.0.0:8080
```

Access `example-shttp-fileserver` via HTTP with `cURL`:

```
curl -v http://127.0.0.1:8080/
```

(Or navigate to http://127.0.0.1:8080/ in a web browser.)

`example-shttp-proxy` can also be used as a proxy from SCION to HTTP, from SCION to SCION, and from HTTP to HTTP. See package [shttp](../../pkg/shttp/README.md) for more details.

### Simple shttp-based server example

Open a shell in the root of the scion-apps repository and run `example-shttp-server`:

```
cd _examples/shttp/server
go run .
```

Open a new shell in the scion-apps repository and access `example-shttp-server` with `scion-bat`:

```
bin/scion-bat 17-ffaa:1:a,[127.0.0.1]:443/hello
```

or

```
bin/scion-bat 17-ffaa:1:a,[127.0.0.1]:443/json
```

or

```
bin/scion-bat -f 17-ffaa:1:a,[127.0.0.1]:443/form foo=bar
```

Run the custom `example-shttp-client` for `example-shttp-server`:

```
bin/example-shttp-client -s 17-ffaa:1:a,[127.0.0.1]:443
```

Run `example-shttp-proxy` to provide `bin/example-shttp-server` functionality via HTTP:

```
bin/example-shttp-proxy --remote=17-ffaa:1:a,[127.0.0.1]:443 --local=0.0.0.0:8080
```

Access `example-shttp-server` via HTTP with `cURL`:

```
curl http://127.0.0.1:8080/hello
```

or

```
curl http://127.0.0.1:8080/json
```

or

```
curl -d foo=bar http://127.0.0.1:8080/form
```

And, finally, to see the cute dog picture:

Navigate to http://127.0.0.1:8080/image in a web browser.
