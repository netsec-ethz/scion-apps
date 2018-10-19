package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/chaehni/scion-http/http"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	rAddr, _ := snet.AddrFromString("17-ffaa:1:c2,[127.0.0.1]:40002")
	lAddr, _ := snet.AddrFromString("17-ffaa:1:c2,[127.0.0.1]:0")

	// Make a map from URL to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["testserver"] = rAddr

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	// Make a get request
	resp, err := c.Get("testserver")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Fatal("Get request failed: ", err)
	}

	log.Println("Get request succeeded:")
	log.Println("Status: ", resp.Status)
	log.Println("Content-Length: ", resp.ContentLength)
	log.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))
}
