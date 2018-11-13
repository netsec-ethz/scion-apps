package main

import (
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/chaehni/scion-http/shttp"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	var remote = flag.String("remote", "", "The address on which the server will be listening")
	var local = flag.String("local", "", "The address on which the server will be listening")
	var interactive = flag.Bool("i", false, "Wether to use interactive mode for path selection")

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

	// Create a standard server with custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Fetching: %v", r.URL.String())
		resp, err := c.Get(r.URL.String())
		if err != nil {
			log.Print("GET request failed: ", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Received status %v", resp.Status)
			http.Error(w, "error fetching file", resp.StatusCode)
		}
		log.Print("Content-Type: ", resp.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Print(err)
		}
	})

	log.Println("Started proxy, listening on localhost:9000")
	err = http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
