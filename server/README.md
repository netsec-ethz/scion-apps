# scionftp server

Server package for FTP + GridFTP extension, adapted to the SCION network.
Forked from [goftp/server](https://gitea.com/goftp/server).

## Usage

To boot a FTP server you will need to provide a driver that speaks to
your persistence layer - the required driver contract is in [the
documentation](https://godoc.org/github.com/elwin/ScionFTP/server#Driver).

Look at the [file driver](../file-driver) to see
an example of how to build a backend.

There is a [sample ftp server](scionftp_server) as a demo. You can build it with this
command:

    go install github.com/elwin/scionFTP/server/scionftp_server

Then run it if you have add $GOPATH to your $PATH:

    scionftp_server -root /tmp

And finally, connect to the server with any FTP client and the following
details:

    host: 127.0.0.1
    port: 2121
    username: admin
    password: 123456

This uses the file driver mentioned above to serve files.


## Contributing

All suggestions and patches welcome, preferably via a git repository I can pull from.
If this library proves useful to you, please let me know.

## Further Reading

There are a range of RFCs that together specify the FTP protocol. In chronological
order, the more useful ones are:

* [http://tools.ietf.org/rfc/rfc959.txt](http://tools.ietf.org/rfc/rfc959.txt)
* [http://tools.ietf.org/rfc/rfc1123.txt](http://tools.ietf.org/rfc/rfc1123.txt)
* [http://tools.ietf.org/rfc/rfc2228.txt](http://tools.ietf.org/rfc/rfc2228.txt)
* [http://tools.ietf.org/rfc/rfc2389.txt](http://tools.ietf.org/rfc/rfc2389.txt)
* [http://tools.ietf.org/rfc/rfc2428.txt](http://tools.ietf.org/rfc/rfc2428.txt)
* [http://tools.ietf.org/rfc/rfc3659.txt](http://tools.ietf.org/rfc/rfc3659.txt)
* [http://tools.ietf.org/rfc/rfc4217.txt](http://tools.ietf.org/rfc/rfc4217.txt)

For an english summary that's somewhat more legible than the RFCs, and provides
some commentary on what features are actually useful or relevant 24 years after
RFC959 was published:

* [http://cr.yp.to/ftp.html](http://cr.yp.to/ftp.html)

For a history lesson, check out Appendix III of RCF959. It lists the preceding
(obsolete) RFC documents that relate to file transfers, including the ye old
RFC114 from 1971, "A File Transfer Protocol"

This library is heavily based on [em-ftpd](https://github.com/yob/em-ftpd), an FTPd
framework with similar design goals within the ruby and EventMachine ecosystems. It
worked well enough, but you know, callbacks and event loops make me something
something.
