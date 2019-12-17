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
// limitations under the License.package main

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
			w.Header().Set("Content-Type", "application/json")
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
