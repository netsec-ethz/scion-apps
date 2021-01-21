# scion-ftpd

This is a sample FTP server for testing and demonstrating usage of FTP on the SCION network. Build this application
from [scion-apps](../../) using the command `make scion-ftpd`

```
$ scionftp_server -h
Usage of scion-ftpd:
  -hercules string
    	Enable RETR_HERCULES using the Hercules binary specified
    	In Hercules mode, scionFTP checks the following directories for Hercules config files: ., /etc, /etc/scion-ftp
  -pass string
    	Password for login (omit for anonymous FTP)
  -port uint
    	Port (default 2121)
  -root string
    	Root directory to serve
  -user string
    	Username for login (omit for anonymous FTP)
```
