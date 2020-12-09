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

package nesquic

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

// ListenPort listens for QUIC connections on a SCION/UDP port.
//
// The underlying connection implicitly sends reply packets via the path over which the last packet for a client (identified by IA,IP:Port) was received.
func ListenPort(port uint16, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {
	sconn, err := appnet.ListenPort(port)
	if err != nil {
		return nil, err
	}
	conn := newReturnPathConn(sconn)
	return quic.Listen(conn, tlsConf, quicConfig)
}

// returnPathConn is a wrapper for a snet.Conn which sends reply packets via
// the path over which the last packet for a client (identified by IA,IP:Port)
// was received.
// NOTE: the reply path map is never pruned, so for long living servers, this
// is a potential memory leak.
type returnPathConn struct {
	net.PacketConn
	mutex sync.RWMutex
	paths map[returnPathKey]returnPath
}

func newReturnPathConn(conn *snet.Conn) *returnPathConn {
	return &returnPathConn{
		PacketConn: conn,
		paths:      make(map[returnPathKey]returnPath),
	}
}

func (c *returnPathConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	for _, ok := err.(*snet.OpError); err != nil && ok; {
		n, addr, err = c.PacketConn.ReadFrom(p)
	}
	if err == nil {
		if saddr, ok := addr.(*snet.UDPAddr); ok {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			c.paths[makeKey(saddr)] = returnPath{path: saddr.Path, nextHop: saddr.NextHop}
			saddr.Path = nil // hide it,
			saddr.NextHop = nil
		}
	}
	return n, addr, err
}

func (c *returnPathConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if saddr, ok := addr.(*snet.UDPAddr); ok && saddr.Path == nil { // XXX && saddr.IA = localIA
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		retPath, ok := c.paths[makeKey(saddr)]
		if ok {
			addr = &snet.UDPAddr{
				IA:      saddr.IA,
				Host:    saddr.Host,
				Path:    retPath.path,
				NextHop: retPath.nextHop,
			}
		}
	}
	return c.PacketConn.WriteTo(p, addr)
}

type returnPath struct {
	path    *spath.Path
	nextHop *net.UDPAddr
}

type returnPathKey struct {
	ia   addr.IA
	ip   [16]byte
	port int
}

func makeKey(addr *snet.UDPAddr) returnPathKey {
	ip := [16]byte{}
	copy(ip[:], addr.Host.IP)
	return returnPathKey{
		ia:   addr.IA,
		ip:   ip,
		port: addr.Host.Port,
	}
}
