# HTTP over SCION/QUIC

This package contains a client/server implementation of HTTP/2 over SCION/QUIC as well as a proxy implementation to proxy HTTP/2 requests over SCION to HTTP/1.1 and vice versa.

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
Hostnames are resolved by parsing the `/etc/hosts` file. Known hosts can be added by adding lines like this:

```
# The following lines are SCION hosts
17-ffaa:1:10,[10.0.8.100]	host1
18-ffaa:0:11,[10.0.8.120]	host2
```

### The Server is a full HTTP/2 server designed to work similar to the standard net/http implementation. It supports:

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
The proxy can handle two directions: From HTTP/1.1 to SCION and from SCION to HTTP/1.1. It's idea is to make resources provided over HTTP accessible over the SCION network. 

To use the proxy, consider the proxy example in _examples. This implementation detects from the format of the `remote` and `local` argument if it should listen on SCION/HTTP/1.1 and proxy to SCION/HTTP/1.1.

```Go
local := flag.String("local", "", "The local HTTP or SCION address on which the server will be listening")
remote := flag.String("remote", "", "The SCION or HTTP address on which the server will be requested")

flag.Parse()

mux := http.NewServeMux()

// parseUDPAddr validates if the address is a SCION address
// which we can use to proxy to SCION
if _, err := snet.ParseUDPAddr(*remote); err == nil {
	mux.Handle("/", shttp.NewSingleSCIONHostReverseProxy(*remote, nil))
	log.Printf("Proxy to SCION remote %s\n", *remote)
} else {
	u, err := url.Parse(*remote)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed parse remote %s, %s", *remote, err))
	}
	log.Printf("Proxy to HTTP remote %s\n", *remote)
	mux.Handle("/", httputil.NewSingleHostReverseProxy(u))
}

if lAddr, err := snet.ParseUDPAddr(*local); err == nil {
	log.Printf("Listen on SCION %s\n", *local)
	// ListenAndServe does not support listening on a complete SCION Address,
	// Consequently, we only use the port (as seen in the server example)
	log.Fatalf("%s", shttp.ListenAndServe(fmt.Sprintf(":%d", lAddr.Host.Port), mux, nil))
} else {
	log.Printf("Listen on HTTP %s\n", *local)
	log.Fatalf("%s", http.ListenAndServe(*local, mux))
}

```

To proxy from SCION to HTTP/1.1, use
`./proxy --local="19-ffcc:1:aaa,[127.0.0.1]:42424" --remote="http://192.168.0.1:8090"`

and to proxy to SCION from HTTP/1.1, use
`./proxy --remote="19-ffcc:1:aaa,[127.0.0.1]:42425" --local="192.168.0.1:8091"`

Furthermore, also proxying from SCION to SCION and from HTTP/1.1 to HTTP/1.1 is possible by entering the correct address formats for SCION and HTTP/1.1 respectively.