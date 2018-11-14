package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/chaehni/scion-http/shttp"
)

func main() {

	var local = flag.String("local", "", "The address on which the server will be listening")
	var tlsCert = flag.String("cert", "tls.pem", "Path to TLS pemfile")
	var tlsKey = flag.String("key", "tls.key", "Path to TLS keyfile")

	flag.Parse()

	http.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		// serve the sample JPG file
		// Status 200 OK will be set implicitly
		// Conent-Length will be inferred by server
		// Content-Type will be detected by server
		http.ServeFile(w, r, "dog.jpg")
	})

	err := shttp.ListenAndServeSCION(*local, *tlsCert, *tlsKey, nil)
	if err != nil {
		log.Fatal(err)
	}

}
