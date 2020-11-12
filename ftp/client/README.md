# scionftp client

Client package for FTP + GridFTP extension, adapted to the SCION network.
Forked from [jlaffaye/ftp](https://github.com/jlaffaye/ftp).

## Example ##

```go
c, err := ftp.Dial("1-ff00:0:110,[127.0.0.1]:4000, 1-ff00:0:110,[127.0.0.1]:2121", ftp.DialWithTimeout(5*time.Second))
if err != nil {
    log.Fatal(err)
}

err = c.Login("admin", "123456")
if err != nil {
    log.Fatal(err)
}

// Do something with the FTP connection

if err := c.Quit(); err != nil {
    log.Fatal(err)
}
```
