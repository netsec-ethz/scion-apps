package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/chaehni/scion-http/http"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	rAddr, _ := snet.AddrFromString("17-ffaa:1:c2,[127.0.0.1]:40002")
	lAddr, _ := snet.AddrFromString("17-ffaa:1:c2,[127.0.0.1]:0")

	// Make a map from URL to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["http://testserver"] = rAddr

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	// Make a get request
	start := time.Now()
	resp, err := c.Get("http://testserver")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	end := time.Now()

	log.Printf("GET request succeeded in %v seconds:", end.Sub(start).Seconds())
	log.Println("Status: ", resp.Status)
	log.Println("Content-Length: ", resp.ContentLength)
	log.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))

	// Make another request

	c = &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	start = time.Now()
	resp, err = c.Post("http://testserver", "text/html", resp.Body)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	end = time.Now()

	log.Printf("2st POST request succeeded in %v seconds:", end.Sub(start).Seconds())
	log.Println("Status: ", resp.Status)
	log.Println("Content-Length: ", resp.ContentLength)
	log.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))
}
