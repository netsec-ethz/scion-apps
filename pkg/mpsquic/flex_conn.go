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

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ net.PacketConn = (*flexConn)(nil)

type flexConn struct {
	net.PacketConn
	raddr   *snet.UDPAddr
	addrMtx sync.RWMutex
}

// newFlexConn returns an initialized flexConn, on which the used
// path can be dynamically updated
func newFlexConn(conn *snet.Conn, raddr *snet.UDPAddr, path snet.Path) *flexConn {
	c := &flexConn{
		PacketConn: conn,
		raddr:      raddr.Copy(),
	}
	appnet.SetPath(c.raddr, path)
	return c
}

// SetRemoteAddr updates the remote address path of the flexConn
// connection in a thread safe manner.
func (c *flexConn) SetPath(path snet.Path) {
	c.addrMtx.Lock()
	defer c.addrMtx.Unlock()
	appnet.SetPath(c.raddr, path)
}

// WriteTo writes the byte slice b to the embedded SCION connection of the
// flexConn. The raddr parameter is ignored and the data is always written
// to the raddr on the connection.  It returns the number of bytes written and
// any write error encountered.
func (c *flexConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	// Ignore param, force use of c.raddr
	c.addrMtx.RLock()
	defer c.addrMtx.RUnlock()
	return c.PacketConn.WriteTo(b, c.raddr)
}

func (c *flexConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.PacketConn.ReadFrom(p)
	// Ignore revocation notifications. These are handled by the revocation handler, we don't need
	// to tell anybody else...
	if _, ok := err.(*snet.OpError); ok {
		err = nil
	}
	return
}
