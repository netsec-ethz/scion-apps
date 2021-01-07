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

package socket

import (
	"github.com/netsec-ethz/scion-apps/ftp/internal/scion"
	"io"
	"time"
)

type Socket interface {
	io.Reader
	io.Writer
	io.Closer
}

type MultiSocket struct {
	*ReaderSocket
	*WriterSocket
	closed bool
}

func (m *MultiSocket) SetDeadline(t time.Time) error {
	panic("implement me")
}

var _ DataSocket = &MultiSocket{}

// Only the scionftp should close the socket
// Sends the closing message
func (m *MultiSocket) Close() error {
	return m.WriterSocket.Close()
}

func (m *MultiSocket) LocalAddress() scion.Address {
	return m.WriterSocket.sockets[0].LocalAddress()
}

func (m *MultiSocket) RemoteAddress() scion.Address {
	return m.WriterSocket.sockets[0].RemoteAddress()
}

var _ DataSocket = &MultiSocket{}

func NewMultiSocket(sockets []DataSocket, maxLength int) *MultiSocket {
	return &MultiSocket{
		NewReadsocket(sockets),
		NewWriterSocket(sockets, maxLength),
		false,
	}
}
