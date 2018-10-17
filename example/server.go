package main

import (
	"log"

	"github.com/chaehni/scion-http/http"
)

func main() {

	server := shttp.Server{}
	server.AddrString = "17-ffaa:1:c2,[127.0.0.1]:40002"
	server.TLSCertFile = "tls.pem"
	server.TLSKeyFile = "tls.key"

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}
