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

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func main() {
	var err error
	// get local and remote addresses from program arguments:
	port := flag.Uint("port", 0, "[Server] local port to listen on")
	remoteAddrStr := flag.String("remote", "", "[Client] Remote (i.e. the server's) SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	flag.Parse()

	if (*port > 0) == (len(*remoteAddrStr) > 0) {
		check(fmt.Errorf("Either specify -port for server or -remote for client"))
	}

	localAddr, err := scionutil.GetLocalhost()
	check(err)
	// initialize SCION
	err = scionutil.InitSCION(localAddr)
	check(err)

	if *port > 0 {
		localAddr.Host.L4 = addr.NewL4UDPInfo(uint16(*port))
		err = runServer(localAddr)
		check(err)
	} else {
		remoteAddr, err := snet.AddrFromString(*remoteAddrStr)
		check(err)
		err = runClient(localAddr, remoteAddr)
		check(err)
	}
}

func runServer(localAddr *snet.Addr) error {
	conn, err := snet.ListenSCION("udp4", localAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	buffer := make([]byte, 16*1024)
	for {
		n, from, err := conn.ReadFromSCION(buffer)
		if err != nil {
			return err
		}
		data := buffer[:n]
		fmt.Printf("Received %s: %s\n", from, data)
	}
}

func runClient(localAddr, remoteAddr *snet.Addr) error {
	conn, err := snet.DialSCION("udp4", localAddr, remoteAddr)
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
		fmt.Println("Fatal error. Exiting.", "err", e)
		os.Exit(1)
	}
}
