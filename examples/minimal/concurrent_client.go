package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/chaehni/scion-http/shttp"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	wg    sync.WaitGroup
	mutex sync.Mutex
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
	t := &shttp.Transport{
		DNS:   dns,
		LAddr: lAddr,
	}

	c := &http.Client{
		Transport: t,
	}

	fmt.Println()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go makeRequest(c, i)
	}
	wg.Wait()
}

func makeRequest(c *http.Client, i int) {
	defer wg.Done()

	start := time.Now()
	resp, err := c.Get("https://testserver.com/download")
	end := time.Now()
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()

	mutex.Lock()
	defer mutex.Unlock()

	log.Printf("GET request %v succeeded in %v seconds", i, end.Sub(start).Seconds())
	printResponse(resp, true)
}

func printResponse(resp *http.Response, hasBody bool) {

	fmt.Println("***Printing Response***")
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
