# Hello QUIC

A simple echo server, demonstrating the use of QUIC over SCION.
The client creates a QUIC stream per echo request. The server reads the
entire stream content and echos it back to the client.

Server:
```
go run helloquic.go -listen 127.0.0.1:1234
```

Client:
```
go run helloquic.go -remote 17-ffaa:1:a,[127.0.0.1]:1234
```

Replace `17-ffaa:1:a` with the address of the AS in which the server is running.
