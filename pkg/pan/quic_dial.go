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

	"github.com/lucas-clemente/quic-go"
)

// QUICSession is a wrapper around quic.Session that always closes the
// underlying conn when closing the session.
// This is needed here because we use quic.Dial, not quic.DialAddr but we want
// the close-the-socket behaviour of quic.DialAddr.
type QUICSession struct {
	quic.Session
	Conn Conn
}

func (s *QUICSession) CloseWithError(code quic.ErrorCode, desc string) error {
	err := s.Session.CloseWithError(code, desc)
	s.Conn.Close()
	return err
}

func (s *QUICSession) SetPolicy(policy Policy) {
	s.Conn.SetPolicy(policy)
}

// QUICEarlySession is a wrapper around quic.EarlySession, analogous to closerSession
type QUICEarlySession struct {
	quic.EarlySession
	Conn Conn
}

func (s *QUICEarlySession) CloseWithError(code quic.ErrorCode, desc string) error {
	err := s.EarlySession.CloseWithError(code, desc)
	s.Conn.Close()
	return err
}

func (s *QUICEarlySession) SetPolicy(policy Policy) {
	s.Conn.SetPolicy(policy)
}

// DialAddr establishes a new QUIC connection to a server at the remote address.
//
// If no path is specified in raddr, DialAddr will choose the first available path,
// analogous to appnet.DialAddr.
// The host parameter is used for SNI.
// The tls.Config must define an application protocol (using NextProtos).
func DialQUIC(ctx context.Context,
	local *net.UDPAddr, remote UDPAddr, policy Policy, selector Selector,
	host string, tlsConf *tls.Config, quicConf *quic.Config) (*QUICSession, error) {

	conn, err := DialUDP(ctx, local, remote, policy, selector)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
	session, err := quic.DialContext(ctx, pconn, remote, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &QUICSession{session, conn}, nil
}

// DialAddrEarly establishes a new 0-RTT QUIC connection to a server. Analogous to DialAddr.
func DialQUICEarly(ctx context.Context,
	local *net.UDPAddr, remote UDPAddr, policy Policy, selector Selector,
	host string, tlsConf *tls.Config, quicConf *quic.Config) (*QUICEarlySession, error) {

	conn, err := DialUDP(ctx, local, remote, policy, selector)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
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
