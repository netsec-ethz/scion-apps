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
	"fmt"
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

func main() {
	port := flag.Uint("p", 443, "port the server listens on")
	flag.Parse()

	handler := http.FileServer(http.Dir(""))
	log.Fatal(shttp.ListenAndServe(fmt.Sprintf(":%d", *port), handler, nil))
}
