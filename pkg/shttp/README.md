# HTTP over SCION

This package contains glue code to use the standard net/http libraries for HTTP
over SCION.

This uses a QUIC session with a single stream as a transport, instead of the
standard TCP (for which we do not have an implementation on top of SCION).
As TLS is always enabled in QUIC, we use an insecure TLS session with self
signed certificates to get something similar to TCP for insecure HTTP.
For HTTPS, we'll have two TLS sessions; the insecure TLS for the basic
transport and on top of that, the "normal" TLS for the actual web content.
This may seem silly, and the net/http library provides enough hooks that would
allow using the "normal" TLS session directly. However, only this setup allows
to implement CONNECT, e.g. to proxy HTTPS traffic over HTTP.

### Client

We use the standard net/http Client/Transport with a customized Dial function:

```Go
// Create a client with our Transport/Dialer:
client := &http.Client{
    Transport: shttp.NewRoundTripper()
}
// Make requests as usual
resp, err := client.Get("http://server:8080/download")
```

Hostnames are resolved by parsing the `/etc/hosts` file or by a RAINS lookup
(see [Hostnames](../../README.md#Hostnames)).
URLs potentially containing raw SCION addresses must be *mangled* before
passing into the client (or any other place where they might be parsed as URL).
```Go
resp, err := client.Get(shttp.MangleSCIONURL("http://1-ff00:0:110,127.0.0.1:8080/download"))
```

### Server

The server is used just like the standard net/http server; the handlers work
all the same, only a custom listener is used for serving.

Example:
```Go
handler := http.FileServer(http.Dir("/usr/share/doc"))))
log.Fatal(shttp.ListenAndServe(":80", handler))
```
