// Copyright 2017 ETH Zurich
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

package mpsquic

import (
	"net"
	"sync"

	"github.com/scionproto/scion/go/lib/snet"
)

var _ net.Conn = (*SCIONFlexConn)(nil)
var _ net.PacketConn = (*SCIONFlexConn)(nil)

// TODO: private
// TODO: rename
type SCIONFlexConn struct {
	*snet.Conn
	raddr   *snet.UDPAddr
	addrMtx sync.RWMutex
}

// newSCIONFlexConn returns an initialized SCIONFlexConn, on which the used
// path can be dynamically updated
func newSCIONFlexConn(conn *snet.Conn, raddr *snet.UDPAddr) *SCIONFlexConn {
	c := &SCIONFlexConn{
		Conn:  conn,
		raddr: raddr,
	}
	return c
}

// SetRemoteAddr updates the remote address raddr of the SCIONFlexConn
// connection in a thread safe manner.
func (c *SCIONFlexConn) SetRemoteAddr(raddr *snet.UDPAddr) {
	c.addrMtx.Lock()
	defer c.addrMtx.Unlock()
	c.setRemoteAddr(raddr)
}

// setRemoteAddr implements the update of the remote address of the SCION connection
func (c *SCIONFlexConn) setRemoteAddr(raddr *snet.UDPAddr) {
	c.raddr = raddr
}

// Write writes the byte slice b to the embedded SCION connection of the SCIONFlexConn.
// It returns the number of bytes written and any write error encountered.
func (c *SCIONFlexConn) Write(b []byte) (n int, err error) {
	c.addrMtx.RLock()
	defer c.addrMtx.RUnlock()
	return c.Conn.WriteTo(b, c.raddr)
}

// WriteTo writes the byte slice b to the embedded SCION connection of the
// SCIONFlexConn. The raddr parameter is ignored and the data is always written
// to the raddr on the connection.  It returns the number of bytes written and
// any write error encountered.
func (c *SCIONFlexConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	// Ignore param, force use of c.raddr
	return c.Write(b)
}
