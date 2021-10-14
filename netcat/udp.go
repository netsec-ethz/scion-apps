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
	"io"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

// May not be accessed from multiple threads concurrently, especially Read(...) and Close(...)
type udpListenConn struct {
	requests  chan<- []byte
	responses <-chan int
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
func DoDialUDP(remote string) (io.ReadWriteCloser, error) {
	return appnet.Dial(remote)
}

// DoListenUDP listens on a UDP socket
func DoListenUDP(port uint16) (chan io.ReadWriteCloser, error) {
	conn, err := appnet.ListenPort(port)
	if err != nil {
		return nil, err
	}

	readRequests := make(map[string](chan []byte))
	readResponses := make(map[string](chan int))

	conns := make(chan io.ReadWriteCloser)

	go func() {
		buf := make([]byte, 65536)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				logError("reading from UDP socket: %v", err)
				close(conns)
				return
			}
			addrStr := addr.String()

			nbufChan, contained := readRequests[addrStr]
			nrespChan := readResponses[addrStr]
			if !contained {
				// create new UDP connection
				logDebug("New UDP connection", "addr", addrStr)
				nbufChan = make(chan []byte)
				nrespChan = make(chan int, 1)

				readRequests[addrStr] = nbufChan
				readResponses[addrStr] = nrespChan

				conns <- &udpListenConn{
					requests:  nbufChan,
					responses: nrespChan,
					isClosed:  false,
					write: func(b []byte) (n int, err error) {
						return conn.WriteTo(b, addr)
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
					logDebug("UDP connection closed with unread data remaining in buffer, discarding it")
					break
				}
				written := copy(nbuf, buf[from:n])
				from += written
				nrespChan <- written
			}
		}
	}()

	return conns, nil
}
