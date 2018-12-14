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
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/netsec-ethz/scion-apps/lib/shttp"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	var remote = flag.String("remote", "", "The address on which the server will be listening")
	var local = flag.String("local", "", "The address on which the server will be listening")
	var interactive = flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	rAddr, err := snet.AddrFromString(*remote)
	lAddr, err2 := snet.AddrFromString(*local)
	if err != nil || err2 != nil {
		log.Fatal(err)
	}

	if *interactive {
		scionutil.ChoosePathInteractive(lAddr, rAddr)
	} else {
		scionutil.ChoosePathByMetric(scionutil.Shortest, lAddr, rAddr)
	}

	// Make a map from URL to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["testserver.com"] = rAddr

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	// Make a get request
	resp, err := c.Get("https://testserver.com/image")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatal("Received status ", resp.Status)
	}

	fmt.Println("Content-Length: ", resp.ContentLength)
	fmt.Println("Content-Type: ", resp.Header.Get("Content-Type"))

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("received.jpg")
	err = jpeg.Encode(out, img, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Image successfully saved to disk")
}
