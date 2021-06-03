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

// burster application
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/bwtester/README.md

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

func main() {
	port := flag.Uint("port", 0, "[Server] Local port to listen on")
	size := flag.Uint("size", 300, "[Client] Burst size")
	payload := flag.Uint("payload", 100, "[Client] Size of each packet in bytes")
	interactive := flag.Bool("interactive", false, "[Client] Select the path interactively")

	remoteAddr := flag.String("remote", "", "[Client] Remote (i.e. the server's) SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	flag.Parse()

	if (*port > 0) == (len(*remoteAddr) > 0) {
		fmt.Println("Either specify -port for server or -remote for client")
		os.Exit(1)
	}

	var err error
	if *port > 0 {
		err = runServer(int(*port))
	} else {
		err = runClient(*remoteAddr, int(*size), int(*payload), *interactive)
	}
	if err != nil {
		fmt.Println("err", err)
		os.Exit(1)
	}
}

func runClient(address string, burstSize int, payloadSize int, interactive bool) error {
	addr, err := appnet.ResolveUDPAddr(address)
	if err != nil {
		return err
	}
	var path snet.Path
	if interactive {
		path, err = appnet.ChoosePathInteractive(addr.IA)
		if err != nil {
			return err
		}
		appnet.SetPath(addr, path)
	} else {
		paths, err := appnet.QueryPaths(addr.IA)
		if err != nil {
			return err
		}
		path = paths[0]
	}

	conn, err := appnet.DialAddr(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("Running client using payload size %v and burst size %v via %v\n", payloadSize, burstSize, path)

	if err != nil {
		return err
	}

	buffer := make([]byte, payloadSize)

	for {
		fmt.Println("Sending burst")
		buffer[0] = 1

		for i := 0; i < burstSize; i++ {
			_, err := conn.Write(buffer)

			if err != nil {
				fmt.Println(err)
			}
		}

		fmt.Println("Sleeping...")

		// Send reset message twice
		time.Sleep(500 * time.Millisecond)

		buffer[0] = 2
		_, err := conn.Write(buffer)
		if err != nil {
			fmt.Println(err)
		}

		time.Sleep(4 * time.Second)

		buffer[0] = 2
		_, errr := conn.Write(buffer)
		if errr != nil {
			fmt.Println(errr)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func runServer(port int) error {
	listener, err := appnet.ListenPort(uint16(port))
	if err != nil {
		return serrors.WrapStr("can't listen:", err)
	}
	defer listener.Close()
	fmt.Printf("%v,%v\n", appnet.DefNetwork().IA, listener.LocalAddr())

	buffer := make([]byte, 16*1024)
	receivedCount := 0
	reset := true
	for {
		count, _, err := listener.ReadFrom(buffer)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if !reset && count > 0 && buffer[0] == 2 {
			fmt.Printf("Received %v packets in burst\n", receivedCount)
			receivedCount = 0
			reset = true
		} else if count > 0 && buffer[0] != 2 {
			receivedCount += 1
			reset = false
		}
	}
}
