package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chaehni/scion-http/http"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	var remote = flag.String("remote", "", "The address on which the server will be listening")
	var local = flag.String("local", "", "The address on which the server will be listening")

	flag.Parse()

	rAddr, _ := snet.AddrFromString(*remote)
	lAddr, _ := snet.AddrFromString(*local)

	// Make a map from URL to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["testserver.com"] = rAddr

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	// Make a get request
	start := time.Now()
	resp, err := c.Get("http://testserver.com/download")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()
	end := time.Now()

	log.Printf("GET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, true)

	// Make another request with a new client
	c = &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	start = time.Now()
	resp, err = c.Post("http://testserver.com/upload", "text/html", strings.NewReader("Sample payload for POST request"))
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	defer resp.Body.Close()
	end = time.Now()

	log.Printf("2st POST request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, false)
}

func printResponse(resp *http.Response, hasBody bool) {

	log.Println("Status: ", resp.Status)
	log.Println("Content-Length: ", resp.ContentLength)
	log.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	if !hasBody {
		fmt.Println("\n\n")
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))
	fmt.Println("\n\n")
}
