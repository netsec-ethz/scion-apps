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
	"flag"
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-apps/lib/shttp"
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
