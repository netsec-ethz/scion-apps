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
	"os/exec"
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

	repeatAfter  bool
	repeatDuring bool

	commandString string

	verboseMode     bool
	veryVerboseMode bool
)

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("netcat [flags] -l port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]")
	fmt.Println("Note that due to the nature of the UDP/QUIC protocols, the server will only notice incoming clients once data has been sent. You can use the -b argument (on both sides) to force clients to send an extra byte which will then be ignored by the server")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -l: Listen mode")
	fmt.Println("  -k: After the connection ended, accept new connections. Requires -l flag. If -u flag is present, requires -c flag. Incompatible with -K flag")
	fmt.Println("  -K: After the connection has been established, accept new connections. Requires -l and -c flags. Incompatible with -k flag")
	fmt.Println("  -c: Instead of piping the connection to stdin/stdout, run the given command using /bin/sh")
	fmt.Println("  -u: UDP mode")
	fmt.Println("  -local: Local SCION address (default localhost)")
	fmt.Println("  -b: Send or expect an extra (throw-away) byte before the actual data")
	fmt.Println("  -tlsKey: TLS key path. Requires -l flag (default: ./key.pem)")
	fmt.Println("  -tlsCert: TLS certificate path. Requires -l flag (default: ./certificate.pem)")
	fmt.Println("  -v: Enable verbose mode")
	fmt.Println("  -vv: Enable very verbose mode")
}

func main() {
	flag.Usage = printUsage
	flag.StringVar(&localAddrString, "local", "", "Local address string")
	flag.StringVar(&quicTLSKeyPath, "tlsKey", "./key.pem", "TLS key path")
	flag.StringVar(&quicTLSCertificatePath, "tlsCert", "./certificate.pem", "TLS certificate path")
	flag.BoolVar(&extraByte, "b", false, "Expect extra byte")
	flag.BoolVar(&listen, "l", false, "Listen mode")
	flag.BoolVar(&udpMode, "u", false, "UDP mode")
	flag.BoolVar(&repeatAfter, "k", false, "Accept new connections after connection end")
	flag.BoolVar(&repeatDuring, "K", false, "Accept multiple connections concurrently")
	flag.StringVar(&commandString, "c", "", "Command")
	flag.BoolVar(&verboseMode, "v", false, "Verbose mode")
	flag.BoolVar(&veryVerboseMode, "vv", false, "Very verbose mode")
	flag.Parse()

	if veryVerboseMode {
		scionlog.SetupLogConsole("debug")
	} else if verboseMode {
		scionlog.SetupLogConsole("info")
	} else {
		scionlog.SetupLogConsole("error")
	}

	tail := flag.Args()
	if !(len(tail) == 1 && listen) && !(len(tail) == 2 && !listen) {
		golog.Panicf("Incorrect number of arguments! Arguments: %v", tail)
	}

	if repeatAfter && repeatDuring {
		golog.Panicf("-k and -K flags are exclusive!")
	}
	if repeatAfter && !listen {
		golog.Panicf("-k flag requires -l flag!")
	}
	if repeatDuring && !listen {
		golog.Panicf("-K flag requires -l flag!")
	}
	if repeatAfter && udpMode && commandString == "" {
		golog.Panicf("-k flag in UDP mode requires -c flag!")
	}
	if repeatDuring && commandString == "" {
		golog.Panicf("-K flag requires -c flag!")
	}

	log.Info("Launching netcat")

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

	var conns chan io.ReadWriteCloser

	if listen {
		conns = doListen(localAddr)
	} else {
		remoteAddr, err := snet.AddrFromString(fmt.Sprintf("%s:%v", remoteAddressString, port))
		if err != nil {
			golog.Panicf("Can't parse remote address %s: %v", remoteAddressString)
		}

		conns = make(chan io.ReadWriteCloser, 1)
		conns <- doDial(localAddr, remoteAddr)
	}

	if repeatAfter {
		isAvailable := make(chan bool, 1)
		for conn := range conns {
			go func() {
				select {
				case isAvailable <- true:
					pipeConn(conn)
					<-isAvailable
				default:
					log.Info("Closing new connection as there's already a connection", "conn", conn)
					conn.Close()
				}
			}()
		}
	} else if repeatDuring {
		for conn := range conns {
			go pipeConn(conn)
		}
	} else {
		conn := <-conns // Pipe the first incoming connection
		go func() {
			for conn := range conns {
				// Reject all other incoming connections
				conn.Close()
			}
		}()
		pipeConn(conn)
	}

	// Note that we don't close the connection currently

	log.Debug("Done, closing now")
}

func pipeConn(conn io.ReadWriteCloser) {
	closeThis := func() {
		log.Debug("Closing connection...", "conn", conn)
		err := conn.Close()
		if err != nil {
			log.Crit("Error closing connection", "conn", conn)
		}
	}

	log.Info("Piping new connection", "conn", conn)

	var reader io.Reader
	var writer io.Writer
	if commandString == "" {
		reader = os.Stdin
		writer = os.Stdout
	} else {
		cmd := exec.Command("/bin/sh", "-c", commandString)
		log.Debug("Created cmd object", "cmd", cmd, "commandString", commandString)
		var err error
		writer, err = cmd.StdinPipe()
		if err != nil {
			log.Crit("Error getting command's stdin pipe", "cmd", cmd, "err", err)
			return
		}
		reader, err = cmd.StdoutPipe()
		if err != nil {
			log.Crit("Error getting command's stdout pipe", "cmd", cmd, "err", err)
			return
		}
		errreader, err := cmd.StderrPipe()
		if err != nil {
			log.Crit("Error getting command's stderr pipe", "cmd", cmd, "err", err)
			return
		}
		go func() {
			io.Copy(os.Stderr, errreader)
		}()
		err = cmd.Start()
		if err != nil {
			log.Crit("Error starting command", "cmd", cmd, "err", err)
			return
		}
		prevCloseThis := closeThis
		closeThis = func() {
			log.Debug("Waiting for command to end...")
			err := cmd.Wait()
			if err != nil {
				log.Warn("Command exited with error", "err", err)
			}
			prevCloseThis()
		}
	}

	var pipesWait sync.WaitGroup
	pipesWait.Add(2)

	go func() {
		_, err := io.Copy(conn, reader)
		log.Debug("Done copying from (std/process) input", "conn", conn, "error", err)
		pipesWait.Done()
	}()
	_, err := io.Copy(writer, conn)
	log.Debug("Done copying to (std/process) output", "conn", conn, "error", err)
	pipesWait.Done()

	pipesWait.Wait()
	closeThis()

	log.Info("Connection closed", "conn", conn)
}

func doDial(localAddr, remoteAddr *snet.Addr) io.ReadWriteCloser {
	var conn io.ReadWriteCloser
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

func doListen(localAddr *snet.Addr) chan io.ReadWriteCloser {
	err := squic.Init(quicTLSKeyPath, quicTLSCertificatePath)
	if err != nil {
		golog.Panicf("Error initializing squic: %v", err)
	}

	var conns chan io.ReadWriteCloser
	if udpMode {
		conns = modes.DoListenUDP(localAddr)
	} else {
		conns = modes.DoListenQUIC(localAddr)
	}

	var nconns chan io.ReadWriteCloser
	if extraByte {
		nconns = make(chan io.ReadWriteCloser, 16)
		go func() {
			for conn := range conns {
				buf := make([]byte, 1)
				_, err := io.ReadAtLeast(conn, buf, 1)
				if err != nil {
					log.Crit("Failed to read extra byte!", "err", err, "conn", conn)
					continue
				}

				log.Debug("Received extra byte", "connection", conn, "extraByte", buf)

				nconns <- conn
			}
		}()
	} else {
		nconns = conns
	}

	return nconns
}
