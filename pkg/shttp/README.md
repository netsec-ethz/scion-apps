# HTTP over SCION/QUIC

This package contains a client/server implementation of HTTP/3 over SCION/QUIC as well as a proxy implementation to proxy HTTP/3 requests over SCION to HTTP/1.1 and vice versa.

### The Client is a standard net/http client with a custom RoundTripper implementation.

First, create a client:
```Go
client := &http.Client{
    Transport: &shttp.NewRoundTripper(tlsCfg, quicCfg)
}
```

where `tlsCfg` and `quicCfg` can both be left `nil`.

Then, make requests as usual:
```Go
resp, err := client.Get("http://server:8080/download")
```
Hostnames are resolved by parsing the `/etc/hosts` file or by a RAINS lookup (see [Hostnames](../../README.md#Hostnames)).

### The Server is a full HTTP/3 server designed to work similar to the standard net/http implementation. It supports:

* concurrent handling of clients
* standard net/http handlers
* standard net/http helpers such as http.ServeFile, http.Error, http.ServeMux, etc
* detection of Content-Type and Content-Length and setting of headers accordingly

First, create a ServeMux():
```Go
mux := http.NewServeMux()
```

Then, create handlers:
```Go
mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
	// Status 200 OK will be set implicitly
	// Content-Length will be inferred by server
	// Content-Type will be detected by server
	http.ServeFile(w, r, "example/sample.html")
})
```
DefaultServeMux is supported. Use it as usual:
```Go
http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
	// handle request
})

http.Handle("/download", handler)
```

Finally, start the server:
```Go
err := shttp.ListenAndServe(local, mux, nil)
if err != nil {
	log.Fatal(err)
}

```
where `local` is the local (UDP)-address of the server.

### Proxy combines the client and server implementation
The proxy can handle two directions: From HTTP/1.1 to SCION and from SCION to HTTP/1.1. Its idea is to make resources provided over HTTP accessible over the SCION network. 

To use the proxy, consider the proxy example in _examples. This implementation detects from the format of the `remote` and `local` argument if it should listen on SCION/HTTP/1.1 and proxy to SCION/HTTP/1.1.

Example code can be found here: [_examples/shttp/proxy](../../_examples/shttp/proxy/main.go)

To proxy from SCION to HTTP/1.1, use
`./proxy --local="19-ffcc:1:aaa,[127.0.0.1]:42424" --remote="http://192.168.0.1:8090"`

and to proxy to SCION from HTTP/1.1, use
`./proxy --remote="19-ffcc:1:aaa,[127.0.0.1]:42425" --local="192.168.0.1:8091"`

Furthermore, also proxying from SCION to SCION and from HTTP/1.1 to HTTP/1.1 is possible by entering the correct address formats for SCION and HTTP/1.1 respectively.
