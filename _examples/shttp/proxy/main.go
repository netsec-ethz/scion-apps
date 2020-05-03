package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	local := flag.String("local", "", "The local HTTP or SCION address on which the server will be listening")
	remote := flag.String("remote", "", "The SCION or HTTP address on which the server will be requested")

	flag.Parse()

	mux := http.NewServeMux()

	// parseUDPAddr validates if the address is a SCION address
	// which we can use to proxy to SCION
	if _, err := snet.ParseUDPAddr(*remote); err == nil {
		proxyHandler, err := shttp.NewSingleSCIONHostReverseProxy(*remote, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			log.Fatalf("Failed to create SCION reverse proxy %s", err)
		}

		mux.Handle("/", proxyHandler)
		log.Printf("Proxy to SCION remote %s\n", *remote)
	} else {
		u, err := url.Parse(*remote)
		if err != nil {
			log.Fatal(fmt.Sprintf("Failed parse remote %s, %s", *remote, err))
		}
		log.Printf("Proxy to HTTP remote %s\n", *remote)
		mux.Handle("/", httputil.NewSingleHostReverseProxy(u))
	}

	if lAddr, err := snet.ParseUDPAddr(*local); err == nil {
		log.Printf("Listen on SCION %s\n", *local)
		// ListenAndServe does not support listening on a complete SCION Address,
		// Consequently, we only use the port (as seen in the server example)
		log.Fatalf("%s", shttp.ListenAndServe(fmt.Sprintf(":%d", lAddr.Host.Port), mux, nil))
	} else {
		log.Printf("Listen on HTTP %s\n", *local)
		log.Fatalf("%s", http.ListenAndServe(*local, mux))
	}
}
