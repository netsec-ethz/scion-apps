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

Replace `17-ffaa:1:a` with your local AS address printed by the server.

## Walkthrough:

This SCION application is very simple, and it demonstrates what is needed to send data using SCION:

1. Parse addresses from string to binary structures.
2. Initialize the SCION library.
3. Obtain a path manager.
4. Obtain paths from source to destination.
5. Obtain a connection using one of these paths.
6. Use that connection to send the data.

You can find these items in the comments of the code.
