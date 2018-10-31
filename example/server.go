package main

import (
	"flag"
	"io/ioutil"
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

	m.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Print(err)
		}
		log.Println("Body: ", string(body))
		w.WriteHeader(http.StatusNoContent)
	})

	m.HandleFunc("/helloworld", func(w http.ResponseWriter, r *http.Request) {
		// serve the sample HTML file
		// Status 200 OK will be set implicitly
		// Conent-Length will be infered by server
		// Content-Type will be detected by server
		http.ServeFile(w, r, "example/sample.html")
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
