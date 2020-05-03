package main

import (
	"flag"
	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	"log"
)

func main() {

	local := flag.String("local", "", "The local HTTP or SCION address on which the server will be listening")
	remote := flag.String("remote", "", "The SCION address on which the server will be requested")
	direction := flag.String("direction", "", "From normal to scion or from scion to normal")
	tlsCert := flag.String("cert", "tls.pem", "Path to TLS pemfile")
	tlsKey := flag.String("key", "tls.key", "Path to TLS keyfile")

	flag.Parse()

	scionProxy, err := shttp.NewSCIONHTTPProxy(shttp.ProxyArgs{
		Direction: *direction,
		Remote:    *remote,
		Local:     *local,
		TlsCert:   tlsCert,
		TlsKey:    tlsKey,
	})

	if err != nil {
		log.Fatal("Failed to setup SCION HTTP Proxy")
	}

	scionProxy.Start()

}
