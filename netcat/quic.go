// Copyright 2019 ETH Zurich
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

package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

var (
	nextProtos = []string{
		// generic "proto" that we use e.g. for HTTP-over-QUIC
		"raw",
		// we accept anything -- use full list of protocol IDs from
		// https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml#alpn-protocol-ids
		"http/0.9",
		"http/1.0",
		"http/1.1",
		"spdy/1",
		"spdy/2",
		"spdy/3",
		"stun.turn",
		"stun.nat-discovery",
		"h2",
		"h2c",
		"webrtc",
		"c-webrtc",
		"ftp",
		"imap",
		"pop3",
		"managesieve",
		"coap",
		"xmpp-client",
		"xmpp-server",
		"acme-tls/1",
		"mqtt",
		"dot",
		"ntske/1",
		"sunrpc",
		"h3",
		"smb",
		"irc",
		"nntp",
		"nnsp",
	}
)

// DoListenQUIC listens on a QUIC socket
func DoListenQUIC(port uint16) (chan io.ReadWriteCloser, error) {
	quicListener, err := pan.ListenQUIC(
		context.Background(),
		&net.UDPAddr{IP: nil, Port: int(port)},
		nil,
		&tls.Config{
			Certificates: appquic.GetDummyTLSCerts(),
			NextProtos:   nextProtos,
		},
		&quic.Config{KeepAlive: true},
	)
	if err != nil {
		return nil, err
	}
	listener := pan.QUICSingleStreamListener{Listener: quicListener}

	conns := make(chan io.ReadWriteCloser)
	go func() {

		for {
			conn, err := listener.Accept()
			if err != nil {
				logError("Can't accept", "err", err)
				continue
			}
			conns <- conn
		}
	}()

	return conns, nil
}

// DoDialQUIC dials with a QUIC socket
func DoDialQUIC(remote string, policy pan.Policy) (io.ReadWriteCloser, error) {
	remoteAddr, err := pan.ResolveUDPAddr(remote)
	if err != nil {
		return nil, err
	}
	sess, err := pan.DialQUIC(
		context.Background(),
		nil,
		remoteAddr,
		policy,
		nil,
		pan.MangleSCIONAddr(remote),
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         nextProtos,
		},
		&quic.Config{KeepAlive: true},
	)
	if err != nil {
		return nil, err
	}

	return pan.NewQUICSingleStream(sess)
}
