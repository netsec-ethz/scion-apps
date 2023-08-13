// Copyright 2021 ETH Zurich
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

	"github.com/quic-go/quic-go"
	"inet.af/netaddr"
)

// QUICSession is a wrapper around quic.Connection that always closes the
// underlying conn when closing the session.
type QUICSession struct {
	quic.Connection
	Conn Conn
}

func (s *QUICSession) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	err := s.Connection.CloseWithError(code, desc)
	s.Conn.Close()
	return err
}

// QUICEarlySession is a wrapper around quic.EarlyConnection, analogous to closerSession
type QUICEarlySession struct {
	quic.EarlyConnection
	Conn Conn
}

func (s *QUICEarlySession) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	err := s.EarlyConnection.CloseWithError(code, desc)
	s.Conn.Close()
	return err
}

// DialQUIC establishes a new QUIC connection to a server at the remote address.
//
// The host parameter is used for SNI.
// The tls.Config must define an application protocol (using NextProtos).
func DialQUIC(ctx context.Context,
	local netaddr.IPPort, remote UDPAddr, policy Policy, selector Selector,
	host string, tlsConf *tls.Config, quicConf *quic.Config) (*QUICSession, error) {

	conn, err := DialUDP(ctx, local, remote, policy, selector)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	session, err := quic.DialContext(ctx, pconn, remote, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &QUICSession{session, conn}, nil
}

// DialQUICEarly establishes a new 0-RTT QUIC connection to a server. Analogous to DialQUIC.
func DialQUICEarly(ctx context.Context,
	local netaddr.IPPort, remote UDPAddr, policy Policy, selector Selector,
	host string, tlsConf *tls.Config, quicConf *quic.Config) (*QUICEarlySession, error) {

	conn, err := DialUDP(ctx, local, remote, policy, selector)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	session, err := quic.DialEarlyContext(ctx, pconn, remote, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &QUICEarlySession{session, conn}, nil
}

// connectedPacketConn wraps a Conn into a PacketConn interface.
// net makes a weird mess of stream/datagram sockets and connected/unconnected
// sockets. meh.
type connectedPacketConn struct {
	net.Conn
}

func (c connectedPacketConn) WriteTo(b []byte, to net.Addr) (int, error) {
	return c.Write(b)
}

func (c connectedPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, err := c.Read(b)
	return n, c.RemoteAddr(), err
}
