package main

import (
	"github.com/chaehni/scion-http/http"
)

func main() {

	server := shttp.Server{}
	server.AddrString = "17-ffaa:1:c2"
	server.TLSCertFile = ""
	server.TLSKeyFile = ""

	server.ListenAndServe()

}
