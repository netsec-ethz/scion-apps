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

package modes

import (
	"io"
	golog "log"

	"github.com/scionproto/scion/go/lib/snet"

	log "github.com/inconshreveable/log15"
)

// May not be accessed from multiple threads concurrently, especially Read(...) and Close(...)
type udpListenConn struct {
	requests  chan<- []byte
	responses <-chan int
	available <-chan bool
	isClosed  bool
	write     func(b []byte) (int, error)
	close     func() error
}

func (conn *udpListenConn) Read(b []byte) (int, error) {
	conn.requests <- b
	return <-conn.responses, nil
}

func (conn *udpListenConn) Write(b []byte) (int, error) {
	return conn.write(b)
}

func (conn *udpListenConn) Close() error {
	return conn.close()
}

// DoDialUDP dials with a UDP socket
func DoDialUDP(localAddr, remoteAddr *snet.Addr) io.ReadWriteCloser {
	conn, err := snet.DialSCION("udp4", localAddr, remoteAddr)
	if err != nil {
		golog.Panicf("Can't dial remote address %v: %v", remoteAddr, err)
	}

	log.Debug("Connected!")

	return conn
}

// DoListenUDP listens on a UDP socket
func DoListenUDP(localAddr *snet.Addr) chan io.ReadWriteCloser {
	conn, err := snet.ListenSCION("udp4", localAddr)
	if err != nil {
		golog.Panicf("Can't listen on address %v: %v", localAddr, err)
	}

	readRequests := make(map[string](chan []byte))
	readResponses := make(map[string](chan int))

	conns := make(chan io.ReadWriteCloser)

	go func() {
		buf := make([]byte, 65536)
		for {
			n, addr, err := conn.ReadFromSCION(buf)
			if err != nil {
				golog.Panicf("Error reading from UDP socket: %v", err)
			}
			addrStr := addr.String()

			nbufChan, contained := readRequests[addrStr]
			nrespChan := readResponses[addrStr]
			if !contained {
				// create new UDP connection
				log.Info("New UDP connection", "addr", addrStr)
				nbufChan = make(chan []byte)
				nrespChan = make(chan int, 1)

				readRequests[addrStr] = nbufChan
				readResponses[addrStr] = nrespChan

				conns <- &udpListenConn{
					requests:  nbufChan,
					responses: nrespChan,
					isClosed:  false,
					write: func(b []byte) (n int, err error) {
						return conn.WriteToSCION(b, addr)
					},
					close: func() (err error) {
						close(nbufChan)
						delete(readRequests, addrStr)
						return nil
					},
				}
			}

			// copy to the correct buffer
			from := 0
			for from < n {
				nbuf, open := <-nbufChan
				if !open {
					log.Debug("UDP connection closed with unread data remaining in buffer, discarding it")
					break
				}
				written := copy(nbuf, buf[from:n])
				from += written
				nrespChan <- written
			}
		}
	}()

	return conns
}
