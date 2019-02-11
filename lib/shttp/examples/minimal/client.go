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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/netsec-ethz/scion-apps/lib/shttp"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	local := flag.String("local", "", "The clients local address")
	interactive := flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	lAddr, err := snet.AddrFromString(*local)
	if err != nil {
		log.Fatal(err)
	}

	rAddr, err := scionutil.GetHostByName("minimal-server")
	if err != nil {
		log.Fatal(err)
	}
	rAddr.Host.L4 = addr.NewL4UDPInfo(40002)
	if *interactive {
		scionutil.ChoosePathInteractive(lAddr, rAddr)
	} else {
		scionutil.ChoosePathByMetric(scionutil.Shortest, lAddr, rAddr)
	}

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			LAddr: lAddr,
		},
	}

	// Make a get request
	start := time.Now()
	resp, err := c.Get("https://minimal-server:40002/download")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()
	end := time.Now()

	log.Printf("\nGET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, true)

	// close the transport to free address/port from dispatcher
	// (just for demonstration on how to use Close. Clients are safe for concurrent use and should be re-used)
	t, _ := c.Transport.(*shttp.Transport)
	t.Close()

	// create a new client using the same address/port combination which is now free again
	c = &http.Client{
		Transport: &shttp.Transport{
			LAddr: lAddr,
		},
	}

	start = time.Now()
	resp, err = c.Post("https://minimal-server:40002/upload", "text/plain", strings.NewReader("Sample payload for POST request"))
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	defer resp.Body.Close()
	end = time.Now()

	log.Printf("POST request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, false)
}

func printResponse(resp *http.Response, hasBody bool) {
	fmt.Println("\n***Printing Response***")
	fmt.Println("Status: ", resp.Status)
	fmt.Println("Protocol:", resp.Proto)
	fmt.Println("Content-Length: ", resp.ContentLength)
	if !hasBody {
		fmt.Print("\n\n")
		return
	}
	fmt.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	fmt.Println("Body: ", string(body))
	fmt.Print("\n\n")
}
