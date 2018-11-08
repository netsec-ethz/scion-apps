# HTTP over SCION/QUIC

This repository contains a client/server implementation of HTTP over SCION/QUIC.

### Setup

SCION infrastructure must be installed and running on your machine. Instructions on how to set this up can be found [here](https://github.com/netsec-ethz/netsec-scion).
Clone the repository and install the dependencies:

```
govendor init
govendor add +e
govendor fetch +m
```

Refer to the individual examples to see how to run them.

### The Client is a standard net/http client with a custom RoundTripper implementation.

First, create a client:
```Go
client := &http.Client{
    Transport: &shttp.Transport{
        DNS: make(map[string]*snet.Addr),
        LAddr: lAddr
    }
}
```

where `DNS` is a map from strings in the format 'example.com' to the associated SCION address and `lAddr` is the local SCION address of the client.

Then, make requests as usual:
```Go
resp, err := client.Get("http://example.com/download)
```


### The Server is designed to work similar to the standard net/http server. It supports:

* concurrent handling of clients
* standard net/http handlers
* standard net/http helpers such as http.ServeFile, http.Error, http.ServeMux, etc
* detection of Content-Type and Content-Length and setting of headers accordingly

First, create a NewServeMux:
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

Finally, create and start the server:
```Go
server := &shttp.Server{
	AddrString:  *local,
	TLSCertFile: *tlsCert,
	TLSKeyFile:  *tlsKey,
	Mux:         mux,
}

err := server.ListenAndServe()
if err != nil {
	log.Fatal(err)
}

```

Where `local` is the local address of the server, `tlsCert` and `tlsKey` are the TLS key and cert files.
