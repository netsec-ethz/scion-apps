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
	"context"
	"io"
	"net/netip"
	"sync"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

type udpListenConn struct {
	requests  chan<- []byte
	responses <-chan int
	mutex     sync.Mutex // protects Read's requests/responses channel from concurrent Close
	write     func(b []byte) (int, error)
	close     func() error
}

func (conn *udpListenConn) Read(b []byte) (int, error) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.requests == nil {
		return 0, io.EOF
	}
	conn.requests <- b
	return <-conn.responses, nil
}

func (conn *udpListenConn) Write(b []byte) (int, error) {
	return conn.write(b)
}

func (conn *udpListenConn) Close() error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	return conn.close()
}

// DoDialUDP dials with a UDP socket
func DoDialUDP(remote string, policy pan.Policy) (io.ReadWriteCloser, error) {
	remoteAddr, err := pan.ResolveUDPAddr(context.TODO(), remote)
	if err != nil {
		return nil, err
	}
	conn, err := pan.DialUDP(context.Background(), netip.AddrPort{}, remoteAddr, pan.WithPolicy(policy))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// DoListenUDP listens on a UDP socket
func DoListenUDP(port uint16) (chan io.ReadWriteCloser, error) {
	conn, err := pan.ListenUDP(
		context.Background(),
		netip.AddrPortFrom(netip.Addr{}, port),
		nil,
		nil,
	)
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
