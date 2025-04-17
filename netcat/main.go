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
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

var (
	extraByte bool
	listen    bool

	udpMode bool

	repeatAfter             bool
	repeatDuring            bool
	shutdownAfterEOF        bool
	shutdownAfterEOFTimeout time.Duration

	commandString string

	verboseMode bool

	interactive bool
	sequence    string
	preference  string
)

func printUsage() {
	fmt.Println("netcat [flags] host-address:port")
	fmt.Println("netcat [flags] -l port")
	fmt.Println("")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]")
	fmt.Println("Note that due to the nature of the UDP/QUIC protocols, the server will only notice incoming clients once data has been sent. You can use the -b argument (on both sides) to force clients to send an extra byte which will then be ignored by the server")
	fmt.Println("")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -l: Listen mode")
	fmt.Println("  -k: After the connection ended, accept new connections. Requires -l flag. If -u flag is present, requires -c flag. Incompatible with -K flag")
	fmt.Println("  -K: After the connection has been established, accept new connections. Requires -l and -c flags. Incompatible with -k flag")
	fmt.Println("  -N: shutdown the network socket after EOF on the input.")
	fmt.Println("  -q: after EOF on stdin, wait the specified duration and then quit. Implies -N.")
	fmt.Println("  -c: Instead of piping the connection to stdin/stdout, run the given command using /bin/sh")
	fmt.Println("  -u: UDP mode")
	fmt.Println("  -b: Send or expect an extra (throw-away) byte before the actual data")
	fmt.Println("  -v: Enable verbose mode")
}

func main() {
	flag.Usage = printUsage
	flag.BoolVar(&extraByte, "b", false, "Expect extra byte")
	flag.BoolVar(&listen, "l", false, "Listen mode")
	flag.BoolVar(&udpMode, "u", false, "UDP mode")
	flag.BoolVar(&repeatAfter, "k", false, "Accept new connections after connection end")
	flag.BoolVar(&repeatDuring, "K", false, "Accept multiple connections concurrently")
	flag.BoolVar(&shutdownAfterEOF, "N", false, "Shutdown the network socket after EOF on the input.")
	flag.DurationVar(&shutdownAfterEOFTimeout, "q", 0, "After EOF on stdin, wait the specified number of seconds and then quit. Implies -N.")
	flag.StringVar(&commandString, "c", "", "Command")
	flag.BoolVar(&interactive, "interactive", false, "Prompt user for interactive path selection")
	flag.StringVar(&sequence, "sequence", "", "Sequence of space separated hop predicates to specify path")
	flag.StringVar(&preference, "preference", "", "Preference sorting order for paths. "+
		"Comma-separated list of available sorting options: "+
		strings.Join(pan.AvailablePreferencePolicies, "|"))
	flag.BoolVar(&verboseMode, "v", false, "Verbose mode")
	flag.Parse()

	tail := flag.Args()
	if len(tail) != 1 {
		expected := "host-address:port"
		if listen {
			expected = "port"
		}
		log.Fatalf("Incorrect number of arguments! Expected %s, got: %v", expected, tail)
	}

	if repeatAfter && repeatDuring {
		log.Fatalf("-k and -K flags are exclusive!")
	}
	if repeatAfter && !listen {
		log.Fatalf("-k flag requires -l flag!")
	}
	if repeatDuring && !listen {
		log.Fatalf("-K flag requires -l flag!")
	}
	if repeatAfter && udpMode && commandString == "" {
		log.Fatalf("-k flag in UDP mode requires -c flag!")
	}
	if repeatDuring && commandString == "" {
		log.Fatalf("-K flag requires -c flag!")
	}

	var conns chan io.ReadWriteCloser

	if listen {
		port, err := strconv.Atoi(tail[0])
		if err != nil {
			printUsage()
			log.Fatalf("Invalid port %s: %v", tail[0], err)
		}
		conns, err = doListen(uint16(port))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		remoteAddr := tail[0]
		policy, err := pan.PolicyFromCommandline(sequence, preference, interactive, "")
		if err != nil {
			log.Fatal(err)
		}
		conn, err := doDial(remoteAddr, policy)
		if err != nil {
			log.Fatal(err)
		}
		conns = make(chan io.ReadWriteCloser, 1)
		conns <- conn
	}

	if repeatAfter {
		isAvailable := make(chan bool, 1)
		for conn := range conns {
			go func(conn io.ReadWriteCloser) {
				select {
				case isAvailable <- true:
					pipeConn(conn)
					<-isAvailable
				default:
					logDebug("Closing new connection as there's already a connection", "conn", conn)
					conn.Close()
				}
			}(conn)
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

	logDebug("Done, closing now")
}

func pipeConn(conn io.ReadWriteCloser) {
	closeThis := func() {
		logDebug("Closing connection...", "conn", conn)
		err := conn.Close()
		if err != nil {
			logError("Error closing connection", "conn", conn)
		}
	}

	logDebug("Piping new connection", "conn", conn)

	var readerDesc, writerDesc string
	var reader io.Reader
	var writer io.WriteCloser
	if commandString == "" {
		reader = os.Stdin
		readerDesc = "stdin"
		writer = os.Stdout
		writerDesc = "stdout"
	} else {
		cmd := exec.Command("/bin/sh", "-c", commandString)
		logDebug("Created cmd object", "cmd", cmd, "commandString", commandString)
		var err error
		writer, err = cmd.StdinPipe()
		if err != nil {
			logError("Error getting command's stdin pipe", "cmd", cmd, "err", err)
			return
		}
		writerDesc = "process input"
		reader, err = cmd.StdoutPipe()
		if err != nil {
			logError("Error getting command's stdout pipe", "cmd", cmd, "err", err)
			return
		}
		readerDesc = "process output"
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil {
			logError("Error starting command", "cmd", cmd, "err", err)
			return
		}
		prevCloseThis := closeThis
		closeThis = func() {
			logDebug("Waiting for command to end...")
			err := cmd.Wait()
			if err != nil {
				logError("Command exited with error", "err", err)
			}
			prevCloseThis()
		}
	}

	var pipesWait sync.WaitGroup
	pipesWait.Add(2)

	go func() {
		_, err := io.Copy(conn, reader)
		logDebug(fmt.Sprintf("Done copying from %s", readerDesc), "conn", conn, "error", err)
		if shutdownAfterEOF || shutdownAfterEOFTimeout > 0 {
			time.Sleep(shutdownAfterEOFTimeout)
			if cw, ok := conn.(interface{ CloseWrite() error }); ok { // quic mode, close stream for writing
				_ = cw.CloseWrite()
			} else {
				_ = conn.Close()
			}
		}
		pipesWait.Done()
	}()
	_, err := io.Copy(writer, conn)
	logDebug(fmt.Sprintf("Done copying to %s", writerDesc), "conn", conn, "error", err)
	pipesWait.Done()

	pipesWait.Wait()
	closeThis()

	logDebug("Connection closed", "conn", conn)
}

func doDial(remoteAddr string, policy pan.Policy) (io.ReadWriteCloser, error) {
	var conn io.ReadWriteCloser
	var err error
	if udpMode {
		conn, err = DoDialUDP(remoteAddr, policy)
	} else {
		conn, err = DoDialQUIC(remoteAddr, policy)
	}
	if err != nil {
		return nil, err
	}
	logDebug("Connected")

	if extraByte {
		_, err := conn.Write([]byte{88}) // ascii('X')
		if err != nil {
			return nil, fmt.Errorf("error writing extra byte: %w", err)
		}
		logDebug("Sent extra byte!")
	}

	return conn, nil
}

func doListen(port uint16) (chan io.ReadWriteCloser, error) {
	var conns chan io.ReadWriteCloser
	var err error
	if udpMode {
		conns, err = DoListenUDP(port)
	} else {
		conns, err = DoListenQUIC(port)
	}
	if err != nil {
		return nil, err
	}

	if extraByte {
		nconns := make(chan io.ReadWriteCloser, 16)
		go func() {
			for conn := range conns {
				buf := make([]byte, 1)
				_, err := io.ReadAtLeast(conn, buf, 1)
				if err != nil {
					logError("Failed to read extra byte!", "err", err, "conn", conn)
					continue
				}

				logDebug("Received extra byte", "connection", conn, "extraByte", buf)

				nconns <- conn
			}
		}()
		return nconns, nil
	} else {
		return conns, nil
	}
}

func logDebug(msg string, ctx ...interface{}) {
	if !verboseMode {
		return
	}
	logWithCtx("DEBUG: ", msg, ctx...)
}

func logError(msg string, ctx ...interface{}) {
	logWithCtx("ERROR: ", msg, ctx...)
}

func logWithCtx(prefix, msg string, ctx ...interface{}) {
	line := prefix + msg
	for i := 0; i < len(ctx); i += 2 {
		line += fmt.Sprintf(" %s=%v", ctx[i], ctx[i+1])
	}
	log.Println(line)
}
