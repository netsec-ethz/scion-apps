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

package scion

import (
	"net"

	"github.com/lucas-clemente/quic-go"
)

var _ net.Conn = &Connection{}

type Connection struct {
	quic.Stream
	local, remote Address
}

func NewAppQuicConnection(stream quic.Stream, local, remote Address) *Connection {
	return &Connection{stream, local, remote}
}

func (conn *Connection) LocalAddr() net.Addr {
	return conn.local
}

func (conn *Connection) RemoteAddr() net.Addr {
	return conn.remote
}

func (conn *Connection) Close() error {
	return conn.Stream.Close()
}

// Contains address with readable port etc
// There should be another way to handle this,
// together with net.Addr
func (conn *Connection) LocalAddress() Address {
	return conn.local
}

func (conn *Connection) RemoteAddress() Address {
	return conn.remote
}
