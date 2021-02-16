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

package striping

import (
	"net"
	"time"
)

type MultiSocket struct {
	*readerSocket
	*writerSocket
	closed bool
}

func (m *MultiSocket) SetReadDeadline(t time.Time) error {
	// readerSocket.sockets contains the same sockets as writerSocket.sockets, hence it's fine to just:
	for _, s := range m.readerSocket.sockets {
		if err := s.SetReadDeadline(t); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiSocket) SetWriteDeadline(t time.Time) error {
	// writerSocket.sockets contains the same sockets as readerSocket.sockets, hence it's fine to just:
	for _, s := range m.writerSocket.sockets {
		if err := s.SetWriteDeadline(t); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiSocket) SetDeadline(t time.Time) error {
	if err := m.SetReadDeadline(t); err != nil {
		return err
	}
	return m.SetWriteDeadline(t)
}

var _ net.Conn = &MultiSocket{}

// Only the client should close the socket
// Sends the closing message
func (m *MultiSocket) Close() error {
	return m.writerSocket.Close()
}

func (m *MultiSocket) LocalAddr() net.Addr {
	return m.writerSocket.sockets[0].LocalAddr()
}

func (m *MultiSocket) RemoteAddr() net.Addr {
	return m.writerSocket.sockets[0].RemoteAddr()
}

func NewMultiSocket(sockets []net.Conn, maxLength int) *MultiSocket {
	return &MultiSocket{
		newReaderSocket(sockets),
		newWriterSocket(sockets, maxLength),
		false,
	}
}
