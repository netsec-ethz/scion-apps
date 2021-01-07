# scion-ftpd

This is a sample FTP server for testing and demonstrating usage of FTP on the SCION network. Build this application
from [scion-apps](../../) using the command `make scion-ftpd`

```
$ scionftp_server -h
Usage of scionftp_server:
  -host string
    	Host (e.g. 1-ff00:0:110,[127.0.0.1])
  -pass string
    	Password for login (default "123456")
  -port int
    	Port (default 2121)
  -root string
    	Root directory to serve
  -user string
    	Username for login (default "admin")

```
