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

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ net.Conn = (*SCIONFlexConn)(nil)
var _ net.PacketConn = (*SCIONFlexConn)(nil)
var _ snet.Conn = (*SCIONFlexConn)(nil)

type SCIONFlexConn struct {
	snet.Conn
	mpq     *MPQuic
	laddr   *snet.Addr
	raddr   *snet.Addr
	addrMtx sync.RWMutex
}

func newSCIONFlexConn(sconn snet.Conn, mpq *MPQuic, laddr, raddr *snet.Addr) *SCIONFlexConn {
	c := &SCIONFlexConn{
		Conn:  sconn,
		mpq:   mpq,
		laddr: laddr,
		raddr: raddr,
	}
	return c
}

// SetRemoteAddr updates the remote address raddr of the SCIONFlexConn connection in a thread safe manner.
func (c *SCIONFlexConn) SetRemoteAddr(raddr *snet.Addr) {
	c.addrMtx.Lock()
	defer c.addrMtx.Unlock()
	c.setRemoteAddr(raddr)
}

func (c *SCIONFlexConn) setRemoteAddr(raddr *snet.Addr) {
	c.raddr = raddr
}

// Write writes the byte slice b to the embedded SCION connection of the SCIONFlexConn.
// It returns the number of bytes written and any write error encountered.
func (c *SCIONFlexConn) Write(b []byte) (n int, err error) {
	return c.WriteToSCION(b, c.raddr)
}

// WriteTo writes the byte slice b to the embedded SCION connection of the SCIONFlexConn. The raddr parameter is ignored
// and the data is always written to the raddr on the connection.
// It returns the number of bytes written and any write error encountered.
func (c *SCIONFlexConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	_, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	// Ignore raddr, force use of c.raddr
	return c.WriteToSCION(b, c.raddr)
}

// WriteToSCION writes the byte slice b to the embedded SCION connection of the SCIONFlexConn. The raddr parameter is ignored
// and the data is always written to the raddr on the connection.
// It returns the number of bytes written and any write error encountered.
func (c *SCIONFlexConn) WriteToSCION(b []byte, raddr *snet.Addr) (int, error) {
	c.addrMtx.RLock()
	defer c.addrMtx.RUnlock()
	// Ignore raddr, force use of c.raddr
	n, err := c.Conn.WriteToSCION(b, c.raddr)
	return n, err
}

func (c *SCIONFlexConn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFromSCION(b)
	return n, err
}

func (c *SCIONFlexConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.ReadFromSCION(b)
	return n, addr, err
}

func (c *SCIONFlexConn) ReadFromSCION(b []byte) (int, *snet.Addr, error) {
	n, addr, err := c.Conn.ReadFromSCION(b)
	return n, addr, err
}
