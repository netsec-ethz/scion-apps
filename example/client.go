package main

import (
	"log"

	"github.com/chaehni/scion-http/http"
)

func main() {

	client := shttp.Client{}
	client.AddrString = "17-ffaa:1:c2,[127.0.0.1]:0"

	response, err := client.Get("17-ffaa:1:c2,[127.0.0.1]:40002")
	if err != nil {
		log.Fatal(err)
	}

	log.Println(response)
}
