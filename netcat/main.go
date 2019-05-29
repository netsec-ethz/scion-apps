// Copyright 2019 ETH Zurich
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
	"io"
	golog "log"
	"os"
	"strconv"
	"sync"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"

	"github.com/netsec-ethz/scion-apps/netcat/modes"
	scionlog "github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"

	log "github.com/inconshreveable/log15"
)

var (
	remoteAddressString string
	port                uint16
	localAddrString     string

	quicTLSKeyPath         string
	quicTLSCertificatePath string

	extraByte bool
	listen    bool

	udpMode bool
)

type anyConn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
}

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("netcat [flags] -l port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]")
	fmt.Println("Note that due to the nature of the UDP/QUIC protocols, the server will only notice incoming clients once data has been sent. You can use the -b argument (on both sides) to force clients to send an extra byte which will then be ignored by the server")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -u: UDP mode")
	fmt.Println("  -local: Local SCION address (default localhost)")
	fmt.Println("  -b: Send or expect an extra (throw-away) byte before the actual data")
	fmt.Println("  -tlsKey: TLS key path. Only allowed with -l flag (default: ./key.pem)")
	fmt.Println("  -tlsCert: TLS certificate path. Only allowed with -l flag (default: ./certificate.pem)")
}

func main() {
	scionlog.SetupLogConsole("debug")

	log.Debug("Launching netcat")

	flag.Usage = printUsage
	flag.StringVar(&remoteAddressString, "local", "", "Local address string")
	flag.StringVar(&quicTLSKeyPath, "tlsKey", "./key.pem", "TLS key path")
	flag.StringVar(&quicTLSCertificatePath, "tlsCert", "./certificate.pem", "TLS certificate path")
	flag.BoolVar(&extraByte, "b", false, "Expect extra byte")
	flag.BoolVar(&listen, "l", false, "Listen mode")
	flag.BoolVar(&udpMode, "u", false, "UDP mode")
	flag.Parse()

	tail := flag.Args()
	if !(len(tail) == 1 && listen) && !(len(tail) == 2 && !listen) {
		printUsage()
		golog.Panicf("Incorrect number of arguments! Arguments: %v", tail)
	}

	remoteAddressString = tail[0]
	port64, err := strconv.ParseUint(tail[len(tail)-1], 10, 16)
	if err != nil {
		printUsage()
		golog.Panicf("Can't parse port string %v: %v", port64, err)
	}
	port = uint16(port64)

	if localAddrString == "" {
		localAddrString, err = scionutil.GetLocalhostString()
		if err != nil {
			golog.Panicf("Error getting localhost: %v", err)
		}
	}

	if listen {
		localAddrString = fmt.Sprintf("%s:%v", localAddrString, port)
	}

	localAddr, err := snet.AddrFromString(localAddrString)
	if err != nil {
		golog.Panicf("Error parsing local address: %v", err)
	}

	// Initialize SCION library
	err = scionutil.InitSCION(localAddr)
	if err != nil {
		golog.Panicf("Error initializing SCION connection: %v", err)
	}

	var conn anyConn

	if listen {
		conn = doListen(localAddr)
	} else {
		remoteAddr, err := snet.AddrFromString(fmt.Sprintf("%s:%v", remoteAddressString, port))
		if err != nil {
			golog.Panicf("Can't parse remote address %s: %v", remoteAddressString)
		}

		conn = doDial(localAddr, remoteAddr)
	}

	close := func() {
		err := conn.Close()
		if err != nil {
			log.Warn("Error closing connection", "error", err)
		}
	}

	var once sync.Once
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		if err != nil {
			log.Warn("Error copying from stdin", "error", err)
		}
		once.Do(close)
		if err != nil {
			golog.Panicf("Copying from stdin failed! %v", err)
		}
	}()
	_, err = io.Copy(os.Stdout, conn)
	if err != nil {
		log.Warn("Error copying to stdout", "error", err)
	}
	once.Do(close)

	log.Debug("Done, closing now")
}

func doDial(localAddr, remoteAddr *snet.Addr) anyConn {
	var conn anyConn
	if udpMode {
		conn = modes.DoDialUDP(localAddr, remoteAddr)
	} else {
		conn = modes.DoDialQUIC(localAddr, remoteAddr)
	}

	if extraByte {
		_, err := conn.Write([]byte{88}) // ascii('X')
		if err != nil {
			golog.Panicf("Error writing extra byte: %v", err)
		}

		log.Debug("Sent extra byte!")
	}

	return conn
}

func doListen(localAddr *snet.Addr) anyConn {
	err := squic.Init(quicTLSKeyPath, quicTLSCertificatePath)
	if err != nil {
		golog.Panicf("Error initializing squic: %v", err)
	}

	var conn anyConn
	if udpMode {
		conn = modes.DoListenUDP(localAddr)
	} else {
		conn = modes.DoListenQUIC(localAddr)
	}

	if extraByte {
		buf := make([]byte, 1)
		io.ReadAtLeast(conn, buf, 1)

		log.Debug("Received extra byte!", "extraByte", buf)
	}

	return conn
}
