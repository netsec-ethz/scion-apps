This example application fetches a JPEG image from the server and saves it to the disk.

To run the example, first start the server like this
```sh
go run server.go -local 17-ffaa:1:c2,[127.0.0.1]:40002 -cert tls.pem -key tls.key
```

Then, start the client:
```sh
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0
```

The local address can be omitted, in that case the application binds to localhost.

For an interactive mode that lets the user choose a path from all available paths add the `-i` flag:
```sh
go run client.go -local 17-ffaa:1:c2,[127.0.0.1]:0 -i
```

Make sure to replace the addresses with your own AS addresses and to set the TLS key/cert paths to point to your `tls.key` and `tls.pem` files.

Also, `image-server` must resolve to the SCION address on which you run the server. You can add `image-server` to your known hosts by adding the following line to `/etc/hosts`. (replace `ISD-AS` and `IP` with your actual address):
```
ISD-AS,[IP] image-server
```