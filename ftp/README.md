# ScionFTP: FTP client on the SCION network

This project aims to show feasibility when implementing an exisiting data transmission protocol on the SCION network. Furthermore, to make use of the multi-path property of SCION, we added the GridFTP extension that allows to send traffic on multiple connections.

## Usage

### Server

To run the server, at least, specify the following options:

```bash
$ scion-ftpserver -host LOCAL_SCION_ADDRESS -cert PATH_TO_TLS_CERT -key PATH_TO_TLS_KEY -root PATH_TO_DIRECTORY
```

Please refer to `scion-ftpserver -help` for more options. 

### Client

Example usage of the client:

```bash
$ scion-ftp
> connect 17-ffaa:1:10,[10.0.8.100]:2121
> login admin 123456
> ls
file_1
file_2
> get file_1 local_path
Received 16028 bytes
> quit
Goodbye
```
