# Hello World

A simple application using SCION that sends one packet from a client to a server.

Server:
```
go run helloworld.go -port 1234
```

Client:
```
go run helloworld.go -remote 17-ffaa:1:a,[127.0.0.1]:1234
```

Replace `17-ffaa:1:a` with the address of the AS in which the server is running.

## Walkthrough:

This SCION application is very simple, and it demonstrates what is needed to send data using SCION:

1. Validate command-line arguments.

Server:
2. Open listener connection (appnet.ListenPort).
3. Read packets from connection.
4. Close listener connection.

Client:
2. Open client connection (appnet.Dial).
3. Write packet to connection.
4. Close client connection.
