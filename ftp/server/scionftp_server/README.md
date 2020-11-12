# scionftp_server

This is a sample scionftp_server for testing and demonstrating usage.
Run `go install` in this directory to install this application in `go/bin`.
Make sure to run this application in the `go/src/github.com/scionproto/scion` directory.
`

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
