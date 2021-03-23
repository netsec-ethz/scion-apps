# Examples for HTTP over SCION/QUIC

This directory contains small example programs that show how HTTP can be used over SCION/QUIC for servers, proxies, and clients:

- server: a server with friendly greetings and other examples
- client: a client that talks to the example server
- fileserver: a server that serves the files from its working directory.
    This includes an example for serving both HTTP and HTTPS.
- proxy: a proxy server that can translate between HTTP and HTTP-over-SCION

See also the package [shttp](../../pkg/shttp/README.md) for the underlaying library code.

## Build:

```
make example-shttp-fileserver \
        example-shttp-proxy \
        example-shttp-server \
        example-shttp-client \
        scion-bat
```

## Running:

All examples require a running SCION endhost stack, i.e. a running SCION dispatcher and SCION daemon. Please refer to '[Running](../../README.md#Running)' in this repository's main README and the [SCIONLab tutorials](https://docs.scionlab.org) to get started.
See '[Environment](../../README.md#Environment)' on how to set the dispatcher and sciond environment variables when e.g. running multiple local ASes.

### Simple server example

Open a shell in the root of the scion-apps repository and run `example-shttp-server`:

```
bin/example-shttp-server
```

Open a new shell and run the custom `example-shttp-client` to interact with the `example-shttp-server`:

```
bin/example-shttp-client -s 17-ffaa:1:a,127.0.0.1
```
Replace '17-ffaa:1:a' with your server's ISD and AS numbers.

Alternatively, we can also use the more generic command line HTTP client
`scion-bat` to interact with the `example-shttp-server`. See also the
application '[bat](../../bat/README.md)' for more details on the cURL-like CLI
tool `scion-bat`.

```
bin/scion-bat 17-ffaa:1:a,127.0.0.1/hello
bin/scion-bat 17-ffaa:1:a,127.0.0.1/json
bin/scion-bat -f 17-ffaa:1:a,127.0.0.1/form foo=bar
```

### File server example

Run `example-shttp-fileserver`:

```
bin/example-shttp-fileserver
```

Access `example-shttp-fileserver` with `scion-bat`:

```
bin/scion-bat http://17-ffaa:1:a,127.0.0.1/
```


### File server example with HTTPS

The file server optionally supports serving via HTTPS.
For this, we need a **hostname** for the server, as a raw SCION address cannot
(currently) be used as the subject of a TLS certificate.
Then, we'll need to create a **key** and obtain **certificate** for our server.
We use a self signed certificate here and we cheat by installing the self
signed certificate to the host's root CA list.

```
# echo "1-ff00:0:111,[127.0.0.1] foo-server" >> /etc/scion/hosts
$ mkdir certs; openssl req -newkey rsa:2048 -nodes -keyout certs/server.key -x509 -days 365 -subj '/CN=foo-server' -addext "subjectAltName = DNS:foo-server" -out certs/server.crt
# cp -n certs/server.crt /etc/ssl/certs/ # for ubuntu/debian etc.
```

Then we provide the key/certs for the server at startup:
```
bin/example-shttp-fileserver -cert certs/server.crt -key certs/server.key
```

And then access it with bat:
```
bin/scion-bat https://foo-server
```

Don't forget to remove `/etc/ssl/certs/server.crt` once you're done.

**Note**: Instead of using a hostname and installing the certificate in the
root CA store, we can also use `scion-bat`'s flag `-insecure=true`, to allow
connections with unchecked certificates. But that's a bit boring, right?


### Proxy example: SCION server, TCP/IP client

The `example-shttp-proxy` is a reverse proxy that can proxy requests on TCP/IP to a SCION web server, or vice versa.

Listen on TCP/IP port 8888 and proxy request to a SCION URL, e.g. start the `example-shttp-server` as described above and then

```
bin/example-shttp-proxy --port 8888 --remote=http://17-ffaa:1:a,127.0.0.1
```

Now we can access `example-shttp-server` via TCP/IP with `cURL`:

```
curl -sfS http://127.0.0.1:8888/hello
curl -sfS http://127.0.0.1:8888/json
curl -sfS -d foo=bar http://127.0.0.1:8888/form
```

And, finally, to see the cute dog picture:

Navigate to http://127.0.0.1:8888/image in a web browser.

### Proxy example: TCP/IP server, SCION client

Listen on SCION port 8888 and proxy request to TCP/IP URL, e.g. https://www.scionlab.org

```
bin/example-shttp-proxy --listen-scion --port 8888 --remote=https://www.scionlab.org
```

Now we can access www.scionlab.org via SCION with `scion-bat` (note the `Host:www.scionlab.org` directive, alternatively we could add a corresponding hostname entry in the hosts file).

```
bin/scion-bat http://17-ffaa:1:a,127.0.0.1:8888/ Host:www.scionlab.org
```

or alternatively

```
bin/scion-bat --proxy http://17-ffaa:1:a,127.0.0.1:8888/ http://www.scionlab.org
```
