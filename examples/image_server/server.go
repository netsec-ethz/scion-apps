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

	m.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		// serve the sample JPG file
		// Status 200 OK will be set implicitly
		// Conent-Length will be infered by server
		// Content-Type will be detected by server
		http.ServeFile(w, r, "examples/image_server/dog.jpg")
	})

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
