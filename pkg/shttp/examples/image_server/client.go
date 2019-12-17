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

	. "github.com/netsec-ethz/scion-apps/pkg/scionutil"
	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {

	var local = flag.String("local", "", "The address on which the server will be listening")
	var interactive = flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	var laddr *snet.Addr
	var err error
	if *local == "" {
		laddr, err = GetLocalhost()
	} else {
		laddr, err = snet.AddrFromString(*local)
	}
	if err != nil {
		log.Fatal(err)
	}

	ia, l3, err := GetHostByName("image-server")
	if err != nil {
		log.Fatal(err)
	}
	l4 := addr.NewL4UDPInfo(40002)
	raddr := &snet.Addr{IA: ia, Host: &addr.AppAddr{L3: l3, L4: l4}}

	if *interactive {
		ChoosePathInteractive(laddr, raddr)
	} else {
		ChoosePathByMetric(Shortest, laddr, raddr)
	}

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			LAddr: laddr,
		},
	}

	// Make a get request
	resp, err := c.Get("https://image-server:40002/image")
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
	if err != nil {
		log.Fatal(err)
	}
	err = jpeg.Encode(out, img, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Image successfully saved to disk")
}
