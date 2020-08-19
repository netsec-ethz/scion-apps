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
	scionlog "github.com/scionproto/scion/go/lib/log"
)

func main() {
	port := flag.Uint("p", 443, "port the server listens on")
	flag.Parse()
	_ = scionlog.Setup(scionlog.Config{Console: scionlog.ConsoleConfig{Level: "crit"}})

	handler := http.FileServer(http.Dir(""))
	log.Fatal(shttp.ListenAndServe(fmt.Sprintf(":%d", *port), withLogger(handler), nil))
}

// withLogger returns a handler that logs requests (after completion) in a simple format:
//	  <time> <remote address> "<request>" <status code> <size of reply>
func withLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrec := &recordingResponseWriter{ResponseWriter: w}
		h.ServeHTTP(wrec, r)

		log.Printf("%s \"%s %s %s/SCION\" %d %d\n",
			r.RemoteAddr,
			r.Method, r.URL, r.Proto,
			wrec.status, wrec.bytes)
	})
}

type recordingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recordingResponseWriter) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *recordingResponseWriter) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.bytes += len(b)
	return r.ResponseWriter.Write(b)
}
