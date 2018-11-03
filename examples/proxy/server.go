package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/chaehni/scion-http/http"
)

func main() {

	var local = flag.String("local", "", "The address on which the server will be listening")
	var tlsKey = flag.String("key", "tls.key", "Path to TLS keyfile")
	var tlsCert = flag.String("cert", "tls.pem", "Path to TLS pemfile")

	flag.Parse()

	m := http.NewServeMux()

	m.Handle("/", http.FileServer(http.Dir("./examples/proxy/public")))

	server := &shttp.Server{
		AddrString:  *local,
		TLSCertFile: *tlsCert,
		TLSKeyFile:  *tlsKey,
		Mux:         m,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}
