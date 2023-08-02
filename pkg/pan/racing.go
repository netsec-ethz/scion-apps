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

package pan

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/quic-go/quic-go"

	"github.com/scionproto/scion/go/lib/snet"
)

/* raceDial dials a quic session on every path and returns the session for
   which the succeeded returned first.
 returns:
 	 the dialed quicConnection,
	 the index of the 'active' Path from 'paths' array that won the race
	 flexConn  which contains the 'winner' Path in its 'raddr'
*/

func raceDialEarly(ctx context.Context, conn net.PacketConn,
	raddr *UDPAddr, host string, paths []*Path,
	tlsConf *tls.Config, quicConf *quic.Config) (quic.EarlyConnection, int, *flexConn, error) {

	conns := make([]*flexConn, len(paths))
	for i, path := range paths {
		conns[i] = newFlexConn(conn, raddr.snetUDPAddr(), path)
	}

	type indexedSessionOrError struct {
		id      int
		session quic.EarlyConnection
		err     error
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan indexedSessionOrError)
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()

	for i := range paths {
		go func(id int) {
			sess, err := quic.DialEarlyContext(ctx, conns[id], raddr, host, tlsConf, quicConf)
			results <- indexedSessionOrError{id, sess, err}
		}(i)
	}

	var firstID int
	var firstSession quic.EarlyConnection
	var errs []error
	for range paths {
		result := <-results
		if result.err == nil {
			if firstSession == nil {
				firstSession = result.session
				firstID = result.id
				cancel() // abort all other Dials
			} else {
				// Dial succeeded without cancelling and not first? Unlucky, just close this session.
				// XXX(matzf) wrong layer; error code is supposed to be application layer
				_ = result.session.CloseWithError(quic.ApplicationErrorCode(0), "")
			}
		} else {
			errs = append(errs, result.err)
		}
	}

	if firstSession != nil {
		return firstSession, firstID, conns[firstID], nil
	} else {
		return nil, 0, nil, errs[0] // return first error (multierr?)
	}
}

var _ net.PacketConn = (*flexConn)(nil)

type flexConn struct {
	net.PacketConn
	raddr   *snet.UDPAddr
	addrMtx sync.RWMutex
}

func (fc *flexConn) Read(p []byte) (int, error) {
	n, _, err := fc.ReadFrom(p)
	return n, err
}

// SetRemoteAddr updates the remote address path of the flexConn
// connection in a thread safe manner.
func (c *flexConn) SetPath(path *Path) {
	c.addrMtx.Lock()
	defer c.addrMtx.Unlock()
	SetPath(c.raddr, path)
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

// newFlexConn returns an initialized flexConn, on which the used
// path can be dynamically updated
func newFlexConn(conn net.PacketConn, raddr *snet.UDPAddr, path *Path) *flexConn {
	c := &flexConn{
		PacketConn: conn,
		raddr:      raddr.Copy(),
	}
	SetPath(c.raddr, path)
	return c
}

// SetPath is a helper function to set the path on an snet.UDPAddr
func SetPath(addr *snet.UDPAddr, path *Path) {
	if path == nil {
		addr.Path = nil
		addr.NextHop = nil
	} else {
		addr.Path = path.ForwardingPath.dataplanePath
		addr.NextHop = &net.UDPAddr{IP: path.ForwardingPath.underlay.IP().IPAddr().IP,
			Port: int(path.ForwardingPath.underlay.Port())}
	}
}
