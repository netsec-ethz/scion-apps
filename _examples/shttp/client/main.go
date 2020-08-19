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
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	scionlog "github.com/scionproto/scion/go/lib/log"
)

func main() {
	serverAddrStr := flag.String("s", "", "Server address (<ISD-AS,[IP]> or <hostname>, optionally with appended <:port>)")
	flag.Parse()
	_ = scionlog.Setup(scionlog.Config{Console: scionlog.ConsoleConfig{Level: "crit"}})

	//mpsquic.SetBasicLogging()

	if len(*serverAddrStr) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: shttp.NewRoundTripper(&tls.Config{InsecureSkipVerify: true}, nil),
	}
	// (just for demonstration on how to use Close. Clients are safe for concurrent use and should be re-used)
	defer c.Transport.(shttp.RoundTripper).Close()

	resources := []string{"welcome.html", "style.css", "favicon.ico", "topology.png"}
	//resources := []string{"cosmos-laundromat-2015_1k.mp4"}
	baseURL := shttp.MangleSCIONAddrURL("https://" + *serverAddrStr + "/")
	for _, r := range resources {
		url := baseURL + r
		start := time.Now()
		resp, err := c.Get(url)
		dt := time.Since(start)
		if err == nil {
			b, _ := ioutil.ReadAll(resp.Body)
			s := float64(len(b)) / 1024.0
			fmt.Printf("GET %s: %d %s (%.1fKB, %s)\n", url, resp.StatusCode, resp.Status, s, dt)
		} else {
			fmt.Printf("GET %s: %v\n", url, err)
		}
	}
}
