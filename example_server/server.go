package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chaehni/scion-http/shttp"
)

func main() {

	var local = flag.String("local", "", "The address on which the server will be listening")
	var tlsCert = flag.String("cert", "tls.pem", "Path to TLS pemfile")
	var tlsKey = flag.String("key", "tls.key", "Path to TLS keyfile")

	flag.Parse()

	/*** start of public routes ***/

	// GET handler that responds with a JSON response
	http.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			data := struct {
				Time    string
				Agent   string
				Proto   string
				Message string
			}{
				Time:    time.Now().Format("2006.01.02 15:04:05"),
				Agent:   r.UserAgent(),
				Proto:   r.Proto,
				Message: "success",
			}
			resp, _ := json.Marshal(data)
			w.Header().Set("Content-Type", "text/json")
			fmt.Fprint(w, string(resp))
		} else {
			http.Error(w, "wrong method: "+r.Method, http.StatusForbidden)
		}
	})

	// POST handler that responds by parsing form values and returns them as string
	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "received following data:\n")
			for s := range r.PostForm {
				fmt.Fprint(w, s, "=", r.PostFormValue(s), "\n")
			}
		} else {
			http.Error(w, "wrong method: "+r.Method, http.StatusForbidden)
		}
	})

	/*** end of public routes ***/

	err := shttp.ListenAndServeSCION(*local, *tlsCert, *tlsKey, nil)
	if err != nil {
		log.Fatal(err)
	}
}
