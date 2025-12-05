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
	"net/netip"

	"github.com/quic-go/quic-go"
)

// QUICConn is a wrapper around quic.Conn that always closes the
// underlying conn when closing the connection.
type QUICConn struct {
	*quic.Conn
	UnderlayConn Conn
}

func (s *QUICConn) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	err := s.Conn.CloseWithError(code, desc)
	s.UnderlayConn.Close()
	return err
}

// DialQUIC establishes a new QUIC connection to a server at the remote address.
//
// The host parameter is used for SNI.
// The tls.Config must define an application protocol (using NextProtos).
func DialQUIC(
	ctx context.Context,
	local netip.AddrPort,
	remote UDPAddr,
	host string,
	tlsConf *tls.Config,
	quicConf *quic.Config,
	connOptions ...ConnOptions,
) (*QUICConn, error) {

	conn, err := DialUDP(ctx, local, remote, connOptions...)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()

	// Enable SNI on underlying TLS connection
	// based on quic-go@v0.41.0/transport.go
	if host != "" {
		h, _, err := net.SplitHostPort(host)
		if err != nil { // This happens if the host doesn't contain a port number.
			tlsConf.ServerName = host
		} else {
			tlsConf.ServerName = h
		}
	}

	session, err := quic.Dial(ctx, pconn, remote, tlsConf, quicConf)
	if err != nil {
		// Close the underlying connection if the QUIC session could not be established.
		pconn.Close()
		return nil, err
	}
	return &QUICConn{Conn: session, UnderlayConn: conn}, nil
}

// DialQUICEarly establishes a new 0-RTT QUIC connection to a server. Analogous to DialQUIC.
func DialQUICEarly(
	ctx context.Context,
	local netip.AddrPort,
	remote UDPAddr,
	host string,
	tlsConf *tls.Config,
	quicConf *quic.Config,
	connOptions ...ConnOptions,
) (*QUICConn, error) {

	conn, err := DialUDP(ctx, local, remote, connOptions...)
	if err != nil {
		return nil, err
	}
	pconn := connectedPacketConn{conn}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	session, err := quic.DialEarly(ctx, pconn, remote, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &QUICConn{Conn: session, UnderlayConn: conn}, nil
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
