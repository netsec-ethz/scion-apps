# Examples for HTTP over SCION/QUIC

This directory contains small example applications that show how HTTP/3 over SCION/QUIC can be used for servers, proxies, and clients.

See also the package [shttp](../../pkg/shttp/README.md).

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

See '[Environment](../../README.md#Environment)' on how to set the dispatcher and sciond environment variables.

Build `scion-bat` as a client for `example-shttp-fileserver`:

```
make scion-bat
```

See also the application [bat](../../bat/README.md).

Access `example-shttp-fileserver` with `scion-bat`:

```
bin/scion-bat 17-ffaa:1:a,[127.0.0.1]:443/
```

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


### Simple server example

Run `example-shttp-server`:

```
cd _examples/shttp/server
go run .
```

Open new shell in the scion-apps repository and access `example-shttp-server` with `scion-bat`:

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
