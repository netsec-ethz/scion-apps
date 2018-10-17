package main

import (
	"github.com/chaehni/scion-http/http"
)

func main() {

	server := shttp.Client{}
	server.AddrString = "17-ffaa:1:c2"
	

	response := server.ListenAndServe()
	log.Println(response)
}
