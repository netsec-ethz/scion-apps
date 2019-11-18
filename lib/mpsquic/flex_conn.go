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

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ net.Conn = (*SCIONFlexConn)(nil)
var _ net.PacketConn = (*SCIONFlexConn)(nil)
var _ snet.Conn = (*SCIONFlexConn)(nil)

type SCIONFlexConn struct {
	snet.Conn
	laddr *snet.Addr
	raddr *snet.Addr
}

func newSCIONFlexConn(sconn snet.Conn, laddr, raddr *snet.Addr) *SCIONFlexConn {
	c := &SCIONFlexConn{
		Conn:  sconn,
		laddr: laddr,
		raddr: raddr,
	}
	return c
}

func (c *SCIONFlexConn) SetRemoteAddr(raddr *snet.Addr) {
	c.raddr = raddr
}

func (c *SCIONFlexConn) Write(b []byte) (n int, err error) {
	return c.WriteToSCION(b, c.raddr)
}
func (c *SCIONFlexConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	_, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	// Ignore raddr, force use of c.raddr
	return c.WriteToSCION(b, c.raddr)
}

func (c *SCIONFlexConn) WriteToSCION(b []byte, raddr *snet.Addr) (int, error) {
	// Ignore raddr, force use of c.raddr
	return c.Conn.WriteToSCION(b, c.raddr)
}
