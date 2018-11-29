package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chaehni/scion-http/shttp"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	remote := flag.String("remote", "", "The address on which the server will be listening")
	local := flag.String("local", "", "The clients local address")
	interactive := flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	rAddr, err := snet.AddrFromString(*remote)
	lAddr, err2 := snet.AddrFromString(*local)
	sciondPath := utils.GetSCIOND()
	dispatcherPath := utils.GetDispatcher()
	if err != nil || err2 != nil {
		log.Fatal(err)
	}

	if *interactive {
		utils.ChoosePath(lAddr, rAddr, sciondPath, dispatcherPath)
	}

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
	resp, err := c.Get("https://testserver.com/download")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()
	end := time.Now()

	log.Printf("\nGET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, true)

	// close the transport to free address/port from dispatcher
	// (just for demonstration on how to use Close. Clients are safe for concurrent use and should be re-used)
	t, _ := c.Transport.(*shttp.Transport)
	t.Close()

	// create a new client using the same address/port combination which is now free again
	c = &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	start = time.Now()
	resp, err = c.Post("https://testserver.com/upload", "text/plain", strings.NewReader("Sample payload for POST request"))
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	defer resp.Body.Close()
	end = time.Now()

	log.Printf("POST request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, false)
}

func printResponse(resp *http.Response, hasBody bool) {
	fmt.Println("\n***Printing Response***")
	fmt.Println("Status: ", resp.Status)
	fmt.Println("Protocol:", resp.Proto)
	fmt.Println("Content-Length: ", resp.ContentLength)
	if !hasBody {
		fmt.Print("\n\n")
		return
	}
	fmt.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	fmt.Println("Body: ", string(body))
	fmt.Print("\n\n")
}
