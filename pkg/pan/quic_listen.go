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
	"net/netip"

	"github.com/quic-go/quic-go"
	"github.com/scionproto/scion/pkg/snet"
)

// ListenQUIC listens for QUIC connections on a SCION/UDP port.
//
// See note on wildcard addresses in the package documentation.
//
// BUG This "leaks" the UDP connection, which is never closed.
func ListenQUIC(
	ctx context.Context,
	local netip.AddrPort,
	selector ReplySelector,
	scmpHandler snet.SCMPHandler,
	tlsConf *tls.Config,
	quicConfig *quic.Config) (*quic.Listener, error) {

	conn, err := ListenUDP(ctx, local, selector, scmpHandler)
	if err != nil {
		return nil, err
	}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	listener, err := quic.Listen(conn, tlsConf, quicConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return listener, nil
}
