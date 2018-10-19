package main

import (
	"log"
	"net/http"

	"github.com/chaehni/scion-http/http"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	// Make a map from URLs to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["testserver"] = snet.AddrFromString("17-ffaa:1:c2,[127.0.0.1]:40002")

	c := &http.Client{
		Transport: &shttp.Transport{
			Dns: dns,
		},
	}

	resp, err := c.Get("testserver")
	if err != nil {
		log.Fatal("Get request failed: ", err)
	}
	log.Println("e")

	log.Println(resp.Body)
}
