# Hello World

A simple application using SCION that sends one packet from a client to a server
which replies back.

Server:
```
go run helloworld.go -listen 127.0.0.1:1234
```

Client:
```
go run helloworld.go -remote 17-ffaa:1:a,[127.0.0.1]:1234
```

Replace `17-ffaa:1:a` with the address of the AS in which the server is running.

## Walkthrough:

This SCION application is very simple, and it demonstrates what is needed to send data using SCION:


Server:
1. Open listener connection (`pan.ListenUDP`).
1. Read packets from connection (`conn.ReadFrom`).
1. Write reply packet (`conn.WriteTo`).
1. Close listener connection.

Client:
1. Open client connection (`pan.Dial`).
1. Write packet to connection (`conn.Write`).
1. Read reply packet (`conn.Read`), with timeout
1. Close client connection.
