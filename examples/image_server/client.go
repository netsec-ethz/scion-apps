package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

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
	resp, err := c.Get("http://testserver.com/image")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatal("Received status ", resp.Status)
	}

	log.Print("Content-Type: ", resp.Header.Get("Content-Type"))

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("received.jpg")
	err = jpeg.Encode(out, img, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Image successfully saved to disk")
}
