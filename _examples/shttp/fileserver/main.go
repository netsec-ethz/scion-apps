// Copyright 2020 ETH Zurich
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

// example-shttp-fileserver is a simple HTTP fileserver that serves all files
// and subdirectories under the current working directory.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

func main() {
	certFile := flag.String("cert", "", "Path to TLS server certificate for optional https")
	keyFile := flag.String("key", "", "Path to TLS server key for optional https")
	flag.Parse()

	handler := handlers.LoggingHandler(
		os.Stdout,
		http.FileServer(http.Dir("")),
	)
	if *certFile != "" && *keyFile != "" {
		go func() { log.Fatal(shttp.ListenAndServeTLS(":443", *certFile, *keyFile, handler)) }()
	}
	log.Fatal(shttp.ListenAndServe(":80", handler))
}
