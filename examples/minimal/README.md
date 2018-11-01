This example application makes two requests from the client to the server.

First, it issues a GET request and downloads an HTML file. Afterwards it sends data to the server via POST.

To run the example, first start the server like this
```Go
go run server.go -local 17-ffaa:1:c2,[127.0.0.1]:40002 -cert tls.pem -key tls.key
```

Then, start the client:
```Go
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0 -remote 17-ffaa:1:c2,[127.0.0.1]:40002
```

For an interactive mode that lets the user choose the path to use from all available paths add the `-i` flag:
```Go
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0 -remote 17-ffaa:1:c2,[127.0.0.1]:40002 -i
```

Make sure to replace the addresses with your own AS addresses and to set the TLS key/cert paths to point to your `tls.key` and `tls.pem` files.