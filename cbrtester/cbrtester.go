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

// cbrtester application
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
	timeout := flag.Uint("timeout", 100, "[Server] Size of gap between subsequent packets that is considered a freeze (in ms)")

	bw := flag.Uint("bw", 1024*1024, "[Client] Rate to send at (in bps)")
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
		err = runServer(int(*port), int64(*timeout))
	} else {
		err = runClient(*remoteAddr, int(*bw), int(*payload), *interactive)
	}
	if err != nil {
		fmt.Println("err", err)
		os.Exit(1)
	}
}

func runClient(address string, desiredThroughput int, payloadSize int, interactive bool) error {
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
	fmt.Printf("Running client using payload size %v and rate %v via %v\n", payloadSize, desiredThroughput, path)

	numberOfPacketsPerSecond := float64(desiredThroughput) / 8. / float64(payloadSize)
	interval := time.Duration(float64(time.Second) / numberOfPacketsPerSecond)

	fmt.Printf("Sending %v packets per second with a gap of %v\n", numberOfPacketsPerSecond, interval)

	buffer := make([]byte, payloadSize)
	copy(buffer, []byte("cbrtester")) // allow easy identification of packets

	for {
		before := time.Now()
		_, err = conn.Write(buffer)
		if err != nil {
			fmt.Println("error writing", err)
		}
		took := time.Since(before)

		sleepAmount := interval - took

		if sleepAmount > 0 {
			time.Sleep(sleepAmount)
		}
	}
}

func runServer(port int, timeout int64) error {
	listener, err := appnet.ListenPort(uint16(port))
	if err != nil {
		return serrors.WrapStr("can't listen:", err)
	}
	defer listener.Close()
	fmt.Printf("%v,%v\n", appnet.DefNetwork().IA, listener.LocalAddr())

	lastReceived := time.Now().Add(999 * time.Hour) // avoids elapsed > timeout in the 1st iter

	buffer := make([]byte, 16*1024)

	start := time.Now()
	started := false
	totalByteCount := 0
	instaByteCount := 0
	lastBWReport := time.Now()
	for {
		count, _, err := listener.ReadFrom(buffer)
		if err != nil {
			fmt.Println(err)
			continue
		}

		totalByteCount += count
		instaByteCount += count

		if !started {
			start = time.Now()
			started = true
		}

		elapsed := time.Since(lastReceived).Milliseconds()
		if elapsed > timeout {
			fmt.Printf("Freeze: %.3f s\n", float64(elapsed)/1000.0)
		}

		if time.Since(lastBWReport) > 5*time.Second {
			fmt.Printf("ave. bandwidth: %.3f kbps, insta. bandwidth: %.3f\n",
				float64(totalByteCount)*8./1024./time.Since(start).Seconds(),
				float64(instaByteCount)*8./1024./float64(time.Since(lastBWReport).Seconds()))

			lastBWReport = time.Now()
			instaByteCount = 0
		}
		lastReceived = time.Now()
	}
}
