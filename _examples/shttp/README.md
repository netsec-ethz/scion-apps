# Examples for HTTP over SCION/QUIC

This directory contains small example programs that show how HTTP/3 over SCION/QUIC can be used for servers, proxies, and clients.

See also the package [shttp](../../pkg/shttp/README.md) for the underlaying library code.

### Preparation:

Clone the repository netsec-ethz/scion-apps:

```
git clone https://github.com/netsec-ethz/scion-apps.git
cd scion-apps
```

### Generic file server example

Build `example-shttp-fileserver`:

```
make example-shttp-fileserver
```

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

Build `example-shttp-proxy` to provide `example-shttp-fileserver` functionality via HTTP:

```
make example-shttp-proxy
```

Run `example-shttp-proxy`:

```
bin/example-shttp-proxy --remote=17-ffaa:1:a,[127.0.0.1]:443 --local=0.0.0.0:8080
```

Access `example-shttp-fileserver` via HTTP with `cURL`:

```
curl -v http://127.0.0.1:8080/
```

(Or navigate to http://127.0.0.1:8080/ in a web browser.)

`example-shttp-proxy` can also be used as a proxy from SCION to HTTP/1.1, from SCION to SCION, and from HTTP/1.1 to HTTP/1.1. See package [shttp](../../pkg/shttp/README.md) for more details.

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

Build custom `example-shttp-client`:

```
make example-shttp-client
```

Run `example-shttp-client`:

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
curl -v -d foo=bar http://127.0.0.1:8080/form
```

And, finally, to see the cute dog picture:

Navigate to http://127.0.0.1:8080/image in a web browser.
