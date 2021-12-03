package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gorilla/handlers"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

func main() {
	port := flag.Uint("port", 80, "port the proxy server listens on.")
	listenSCION := flag.Bool("listen-scion", false, "proxy server listens on SCION.")
	remote := flag.String("remote", "", "remote URL to which requests will be forwarded."+
		"Requests are sent over SCION iff this contains a SCION address.")

	flag.Parse()

	if *remote == "" {
		flag.Usage()
		os.Exit(2)
	}
	remoteMangled := shttp.MangleSCIONAddrURL(*remote)
	remoteURL, err := url.Parse(remoteMangled)
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(remoteURL)
	if remoteMangled != *remote {
		proxy.Transport = shttp.DefaultTransport
		log.Printf("Proxy to SCION remote %s\n", remoteURL)
	} else {
		log.Printf("Proxy to IP/TCP remote %s\n", remoteURL)
	}
	handler := handlers.LoggingHandler(os.Stdout, proxy)

	local := fmt.Sprintf(":%d", *port)
	if *listenSCION {
		log.Printf("Listen on SCION %s\n", local)
		// ListenAndServe does not support listening on a complete SCION Address,
		// Consequently, we only use the port (as seen in the server example)
		log.Fatal(shttp.ListenAndServe(local, handler))
	} else {
		log.Printf("Listen on IP/TCP %s\n", local)
		log.Fatal(http.ListenAndServe(local, handler))
	}
}
