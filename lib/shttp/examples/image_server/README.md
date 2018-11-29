This example application fetches a JPEG image from the server and saves it to the disk.

To run the example, first start the server like this
```Go
go run server.go -local 17-ffaa:1:c2,[127.0.0.1]:40002 -cert tls.pem -key tls.key
```

Then, start the client:
```Go
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0 -remote 17-ffaa:1:c2,[127.0.0.1]:40002
```

For an interactive mode that lets the user choose a path from all available paths add the `-i` flag:
```Go
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0 -remote 17-ffaa:1:c2,[127.0.0.1]:40002 -i
```

Make sure to replace the addresses with your own AS addresses and to set the TLS key/cert paths to point to your `tls.key` and `tls.pem` files.