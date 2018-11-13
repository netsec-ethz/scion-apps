package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lucas-clemente/quic-go/h2quic"

	"github.com/scionproto/scion/go/lib/snet/squic"

	"github.com/lucas-clemente/quic-go"

	"github.com/chaehni/scion-http/shttp"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
)

var remote string
var local string

func main() {

	remote = *flag.String("remote", "", "The address on which the server will be listening")
	local = *flag.String("local", "", "The address on which the server will be listening")
	var interactive = flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	rAddr, err := snet.AddrFromString(remote)
	lAddr, err2 := snet.AddrFromString(local)
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

	log.Printf("GET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, true)

	// Make another request with a new client
	/* c = &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	} */

	c = &http.Client{
		Transport: &h2quic.RoundTripper{
			Dial: dial,
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
		fmt.Print("\n\n")
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))
	fmt.Print("\n\n")
}

func dial(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {

	rAddr, err := snet.AddrFromString(remote)
	lAddr, err2 := snet.AddrFromString(local)

	if snet.DefNetwork == nil {
		snet.Init(lAddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
	}

	return squic.DialSCION(nil, lAddr, rAddr)
}
