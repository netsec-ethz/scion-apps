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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

func main() {
	serverAddrStr := flag.String("s", "", "Server address (<ISD-AS,[IP]> or <hostname>, optionally with appended <:port>)")
	flag.Parse()

	if len(*serverAddrStr) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: shttp.NewRoundTripper(nil, nil),
	}
	// (just for demonstration on how to use Close. Clients are safe for concurrent use and should be re-used)
	defer c.Transport.(shttp.RoundTripper).Close()

	// Make a get request
	start := time.Now()
	query := fmt.Sprintf("https://%s/hello", *serverAddrStr)
	resp, err := c.Get(shttp.MangleSCIONAddrURL(query))
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()
	end := time.Now()

	log.Printf("\nGET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp)

	start = time.Now()
	query = fmt.Sprintf("https://%s/form", *serverAddrStr)
	resp, err = c.Post(
		shttp.MangleSCIONAddrURL(query),
		"application/x-www-form-urlencoded",
		strings.NewReader("surname=threepwood&firstname=guybrush"),
	)
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	defer resp.Body.Close()
	end = time.Now()

	log.Printf("POST request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp)
}

func printResponse(resp *http.Response) {
	fmt.Println("\n***Printing Response***")
	fmt.Println("Status: ", resp.Status)
	fmt.Println("Protocol:", resp.Proto)
	fmt.Println("Content-Length: ", resp.ContentLength)
	fmt.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	if len(body) != 0 {
		fmt.Println("Body: ", string(body))
	}
	fmt.Print("\n\n")
}
