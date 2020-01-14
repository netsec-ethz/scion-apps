# HTTP over SCION/QUIC

This package contains a client/server implementation of HTTP/2 over SCION/QUIC.

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
err := shttp.ListenAndServe(local, mux)
if err != nil {
	log.Fatal(err)
}

```
where `local` is the local (UDP)-address of the server.
