This example application makes two requests from the client to the server.

First, it issues a GET request and requests a greeting message. Afterwards it sends data to the server via POST.

Start the example server:
```sh
go run ./pkg/shttp/_examples/server -p 443
```

Then, run the example client, specifying the server's address with the -s flag, e.g.
```sh
go run ./pkg/shttp/_examples/client -s 17-ffaa:1:c2,[127.0.0.1]:443
```
