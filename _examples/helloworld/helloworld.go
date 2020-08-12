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
	"os"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

func main() {
	var err error
	// get local and remote addresses from program arguments:
	port := flag.Uint("port", 0, "[Server] local port to listen on")
	remoteAddr := flag.String("remote", "", "[Client] Remote (i.e. the server's) SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	flag.Parse()

	if (*port > 0) == (len(*remoteAddr) > 0) {
		check(fmt.Errorf("either specify -port for server or -remote for client"))
	}

	if *port > 0 {
		err = runServer(uint16(*port))
		check(err)
	} else {
		err = runClient(*remoteAddr)
		check(err)
	}
}

func runServer(port uint16) error {
	conn, err := appnet.ListenPort(port)
	if err != nil {
		return err
	}
	defer conn.Close()

	buffer := make([]byte, 16*1024)
	for {
		n, from, err := conn.ReadFrom(buffer)
		if err != nil {
			return err
		}
		data := buffer[:n]
		fmt.Printf("Received %s: %s\n", from, data)
	}
}

func runClient(address string) error {
	conn, err := appnet.Dial(address)
	if err != nil {
		return err
	}
	defer conn.Close()

	nBytes, err := conn.Write([]byte("hello world"))
	if err != nil {
		return err
	}
	fmt.Printf("Done. Wrote %d bytes.\n", nBytes)
	return nil
}

// Check just ensures the error is nil, or complains and quits
func check(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, "Fatal error. Exiting.", "err", e)
		os.Exit(1)
	}
}
