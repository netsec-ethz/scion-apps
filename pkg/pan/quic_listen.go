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

// closerListener is a wrapper around quic.Listener that always closes the
// underlying conn when closing the session.
type closerListener struct {
	quic.Listener
	conn net.PacketConn
}

func (l closerListener) Close() error {
	err := l.Listener.Close()
	l.conn.Close()
	return err
}

// ListenPort listens for QUIC connections on a SCION/UDP port.
//
// See note on wildcard addresses in the appnet package documentation.
func ListenQUIC(ctx context.Context, local *net.UDPAddr, selector ReplySelector,
	tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {

	conn, err := ListenUDP(ctx, local, selector)
	if err != nil {
		return nil, err
	}
	listener, err := quic.Listen(conn, tlsConf, quicConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return closerListener{
		Listener: listener,
		conn:     conn,
	}, nil
}
