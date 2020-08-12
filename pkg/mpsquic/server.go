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

package mpsquic

import (
	"crypto/tls"
	"io"
	"net"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

// ListenPort listens for QUIC connections on a SCION/UDP port.
//
// The underlying connection implicitly sends reply packets via the path over which the last packet for a client (identified by IA,IP:Port) was received.
func ListenPort(port uint16, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {

	conn, err := PacketListen(&net.UDPAddr{Port: int(port)}, quicConfig)
	if err != nil {
		return nil, err
	}
	return quic.Listen(conn, tlsConf, quicConfig)
}

// PacketListen creates a wrapped packet conn for QUIC listening. ugh
func PacketListen(addr *net.UDPAddr, quicConfig *quic.Config) (net.PacketConn, error) {
	sconn, err := appnet.Listen(addr)
	if err != nil {
		return nil, err
	}
	return newReturnPathConn(sconn, quicConfig), nil
}

// returnPathConn is a wrapper for a snet.Conn which sends reply packets via
// the path over which the last packet for a client (identified by IA,IP:Port)
// was received.
// NOTE: the reply path map is never pruned, so for long living servers, this
// is a potential memory leak.
type returnPathConn struct {
	net.PacketConn
	connIDLen     int
	mutex         sync.RWMutex
	paths         map[connectionID]returnPath   // clientConnID -> return path
	clientConnIDs map[connectionID]connectionID // serverConnID -> clientConnID
}

// udpAddrEx is a wrapper around snet.UDPAddr, that additionally stores
type udpAddrEx struct {
	snet.UDPAddr
	clientConnID connectionID
	serverConnID connectionID
}

func newReturnPathConn(conn *snet.Conn, quicConfig *quic.Config) *returnPathConn {
	connIDLen := 4 // default, from quic.Config docs
	if quicConfig != nil && quicConfig.ConnectionIDLength != 0 {
		connIDLen = quicConfig.ConnectionIDLength
	}
	return &returnPathConn{
		PacketConn:    conn,
		connIDLen:     connIDLen,
		paths:         make(map[connectionID]returnPath),
		clientConnIDs: make(map[connectionID]connectionID),
	}
}

func (c *returnPathConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	for _, ok := err.(*snet.OpError); err != nil && ok; {
		n, addr, err = c.PacketConn.ReadFrom(p)
	}

	if err != nil {
		return n, addr, err
	}

	clientConnID, err := parseSrcConnectionID(p)
	if err != nil {
		shortHeaderConnIDLen := 4 // from quic config
		serverConnID, err := parseDestConnectionID(p, shortHeaderConnIDLen)
		if err == nil {
			clientConnID = c.clientConnIDs[serverConnID]
		}
	}
	saddr := &udpAddrEx{
		UDPAddr:      *addr.(*snet.UDPAddr),
		clientConnID: clientConnID,
	}

	if (clientConnID != connectionID{}) { // expected case, otherwise just hope for the best...
		c.mutex.Lock()
		c.paths[clientConnID] = returnPath{path: saddr.Path, nextHop: saddr.NextHop}
		c.mutex.Unlock()
		saddr.Path = nil // hide it,
		saddr.NextHop = nil
	}
	return n, saddr, nil
}

func (c *returnPathConn) WriteTo(p []byte, addr net.Addr) (int, error) {

	saddr := addr.(*udpAddrEx)

	if (saddr.serverConnID == connectionID{}) {
		serverConnID, err := parseSrcConnectionID(p)
		if err == nil {
			saddr.serverConnID = serverConnID
			c.mutex.Lock()
			c.clientConnIDs[serverConnID] = saddr.clientConnID
			c.mutex.Unlock()
		}
	}

	if saddr.Path == nil {
		c.mutex.RLock()
		retPath, ok := c.paths[saddr.clientConnID]
		c.mutex.RUnlock()
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

type connectionID = [18]byte

func makeConnectionID(id []byte) connectionID {
	v := connectionID{}
	copy(v[:], id)
	return v
}

// parseDestConnectionID parses the destination connection ID of a packet.
func parseDestConnectionID(data []byte, shortHeaderConnIDLen int) (connectionID, error) {
	if len(data) == 0 {
		return connectionID{}, io.EOF
	}
	isLongHeader := data[0]&0x80 > 0
	if !isLongHeader {
		if len(data) < shortHeaderConnIDLen+1 {
			return connectionID{}, io.EOF
		}
		return makeConnectionID(data[1 : 1+shortHeaderConnIDLen]), nil
	}
	if len(data) < 6 {
		return connectionID{}, io.EOF
	}
	destConnIDLen := int(data[5])
	if len(data) < 6+destConnIDLen {
		return connectionID{}, io.EOF
	}
	return makeConnectionID(data[6 : 6+destConnIDLen]), nil
}

// parseSrcConnectionID parses the source connection ID of a packet.
// The source connection ID is only available on long header packets.
func parseSrcConnectionID(data []byte) (connectionID, error) {
	if len(data) == 0 {
		return connectionID{}, io.EOF
	}
	isLongHeader := data[0]&0x80 > 0
	if !isLongHeader {
		return connectionID{}, io.EOF
	}
	offset := 1
	offset += 4 // skip version
	if len(data) < offset+1 {
		return connectionID{}, io.EOF
	}
	destConnIDLen := int(data[offset])
	offset += 1
	offset += destConnIDLen

	if len(data) < offset+1 {
		return connectionID{}, io.EOF
	}
	srcConnIDLen := int(data[offset])
	offset += 1
	if len(data) < offset+srcConnIDLen {
		return connectionID{}, io.EOF
	}
	return makeConnectionID(data[offset : offset+srcConnIDLen]), nil
}
