# scion-ftpd

This is a sample FTP server for testing and demonstrating usage of FTP on the SCION network. Build this application
from [scion-apps](../../) using the command `make scion-ftpd`

```
$ scionftp_server -h
Usage of scion-ftpd:
  -cert string
    	TLS certificate file
  -hercules string
    	Enable RETR_HERCULES using the Hercules binary specified
  -host string
    	Host (e.g. 1-ff00:0:110,[127.0.0.1])
  -key string
    	TLS private key file
  -pass string
    	Password for login (default "123456")
  -port uint
    	Port (default 2121)
  -root string
    	Root directory to serve
  -user string
    	Username for login (default "admin")
```
