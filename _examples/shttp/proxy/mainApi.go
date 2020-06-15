package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	"github.com/scionproto/scion/go/lib/snet"
)

var mux *http.ServeMux
var proxy *httputil.ReverseProxy

// Can be overwritte by api calls
type ProxyConfig struct {
	Remote string `json:"remote"`
}

//TODO: Thread safe/Routine safe
func setProxyConfig(wr http.ResponseWriter, r *http.Request) {
	var newConfig ProxyConfig

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(wr, err.Error(), http.StatusBadRequest)
		return
	}

	log.Println("Perform config update")
	setRemote(&newConfig.Remote)
}

func proxyWrapper(rw http.ResponseWriter, req *http.Request) {
	proxy.ServeHTTP(rw, req)
}

func setRemote(remote *string) {
	// parseUDPAddr validates if the address is a SCION address
	// which we can use to proxy to SCION
	if _, err := snet.ParseUDPAddr(*remote); err == nil {
		proxy, err = shttp.NewSingleSCIONHostReverseProxy(*remote, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			log.Fatalf("Failed to create SCION reverse proxy %s", err)
		}

		log.Printf("Proxy to SCION remote %s\n", *remote)
	} else {
		u, err := url.Parse(*remote)
		if err != nil {
			log.Fatal(fmt.Sprintf("Failed parse remote %s, %s", *remote, err))
		}
		log.Printf("Proxy to HTTP remote %s\n", *remote)
		proxy = httputil.NewSingleHostReverseProxy(u)
	}
}

func main() {

	local := flag.String("local", "", "The local HTTP or SCION address on which the server will be listening")
	remote := flag.String("remote", "", "The SCION or HTTP address on which the server will be requested")

	flag.Parse()
	mux = http.NewServeMux()

	setRemote(remote)
	mux.HandleFunc("/__api/setconfig", setProxyConfig)
	mux.HandleFunc("/", proxyWrapper)

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
