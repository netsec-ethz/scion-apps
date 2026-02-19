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
	"github.com/scionproto/scion/pkg/snet"
)

// QUICListener is a wrapper around a quic.Listener that also holds the underlying
// net.PacketConn. This is necessary because quic.Listener does not expose the
// underlying connection, which is needed to close it.
type QUICListener struct {
	*quic.Listener
	Conn net.PacketConn
}

func (l *QUICListener) Close() error {
	err := l.Listener.Close()
	l.Conn.Close()
	return err
}

// ListenQUIC listens for QUIC connections on a SCION/UDP port.
//
// See note on wildcard addresses in the package documentation.
func ListenQUIC(
	ctx context.Context,
	as ASContext,
	listen *snet.UDPAddr,
	tlsConf *tls.Config,
	quicConfig *quic.Config,
	selector ReplySelector,
) (*quic.EarlyListener, error) {
	conn, err := ListenUDP(ctx, as, listen.Host.AddrPort(), selector)
	if err != nil {
		return nil, err
	}

	listener, err := quic.ListenEarly(
		conn,
		tlsConf,
		quicConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return listener, nil
}
