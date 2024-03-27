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
	"context"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

func main() {
	var err error
	// get local and remote addresses from program arguments:
	var listen pan.IPPortValue
	flag.Var(&listen, "listen", "[Server] local IP:port to listen on")
	remoteAddr := flag.String("remote", "", "[Client] Remote (i.e. the server's) SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	count := flag.Uint("count", 1, "[Client] Number of messages to send")
	flag.Parse()

	if (listen.Get().Port() > 0) == (len(*remoteAddr) > 0) {
		check(fmt.Errorf("either specify -listen for server or -remote for client"))
	}

	if listen.Get().Port() > 0 {
		err = runServer(listen.Get())
		check(err)
	} else {
		err = runClient(*remoteAddr, int(*count))
		check(err)
	}
}

func runServer(listen netip.AddrPort) error {
	sock, err := pan.NewScionSocket(context.Background(), listen)

	if err != nil {
		return err
	}
	defer sock.Close()
	fmt.Println(sock.LocalAddr())

	buffer := make([]byte, 16*1024)
	for {
		n, from, err := sock.ReadFrom(buffer)
		if err != nil {
			return err
		}
		data := buffer[:n]
		fmt.Printf("Received %s: %s\n", from, data)
		msg := fmt.Sprintf("take it back! %s", time.Now().Format("15:04:05.0"))
		n, err = sock.WriteTo([]byte(msg), from)
		if err != nil {
			return err
		}
		fmt.Printf("Wrote %d bytes.\n", n)
	}
}

func runClient(address string, count int) error {
	addr, err := pan.ResolveUDPAddr(context.TODO(), address)
	if err != nil {
		return err
	}

	sock, err := pan.NewScionSocket(context.Background(), netip.AddrPort{})
	if err != nil {
		return err
	}
	defer sock.Close()

	for i := 0; i < count; i++ {
		nBytes, err := sock.WriteTo([]byte(fmt.Sprintf("hello world %s", time.Now().Format("15:04:05.0"))), addr)
		if err != nil {
			return err
		}
		fmt.Printf("Wrote %d bytes.\n", nBytes)

		buffer := make([]byte, 16*1024)
		/* if err = conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		} */
		n, _, err := sock.ReadFrom(buffer)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		} else if err != nil {
			return err
		}
		data := buffer[:n]
		fmt.Printf("Received reply: %s\n", data)
	}
	return nil
}

// Check just ensures the error is nil, or complains and quits
func check(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, "Fatal error:", e)
		os.Exit(1)
	}
}
