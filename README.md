# HTTP over SCION/QUIC

This repository contains a client/server implementation of HTTP/2 over SCION/QUIC.

### Setup

SCION infrastructure must be installed and running on your machine. Instructions on how to set this up can be found [here](https://github.com/netsec-ethz/netsec-scion).
Clone the repository and install the dependencies:

```
govendor sync
govendor add +e
```

Refer to the individual examples to see how to run them.

### The Client is a standard net/http client with a custom RoundTripper implementation.

First, create a client:
```Go
client := &http.Client{
    Transport: &shttp.Transport{
        DNS: make(map[string]*snet.Addr),
        LAddr: lAddr,
    },
}
```
where `DNS` is a map from strings in the format 'example.com' to the associated SCION address and `lAddr` is the local SCION address of the client.

Then, make requests as usual:
```Go
resp, err := client.Get("http://example.com/download)
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
	// Conent-Length will be inferred by server
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
err := server.ListenAndServeSCION(local, tlsCert, tlsKey)
if err != nil {
	log.Fatal(err)
}

```
where `local` is the local address of the server, `tlsCert` and `tlsKey` are the TLS key and cert files.
