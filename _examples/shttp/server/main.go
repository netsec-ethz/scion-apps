// Copyright 2018 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

func main() {

	port := flag.Uint("p", 443, "port the server listens on")
	flag.Parse()

	m := http.NewServeMux()

	// handler that responds with a friendly greeting
	m.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		// Status 200 OK will be set implicitly
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(`Oh, hello!`))
	})

	// handler that responds with an image file
	m.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		// serve the sample JPG file
		// Status 200 OK will be set implicitly
		// Content-Length will be inferred by server
		// Content-Type will be detected by server
		http.ServeFile(w, r, "dog.jpg")
	})

	// GET handler that responds with some json data
	m.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
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
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, string(resp))
		} else {
			http.Error(w, "wrong method: "+r.Method, http.StatusForbidden)
		}
	})

	// POST handler that responds by parsing form values and returns them as string
	m.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
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

	log.Fatal(shttp.ListenAndServe(fmt.Sprintf(":%d", *port), m))
}
