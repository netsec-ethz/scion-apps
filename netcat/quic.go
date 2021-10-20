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

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
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

type sessConn struct {
	sess   quic.Session
	stream quic.Stream
}

func (conn *sessConn) Read(b []byte) (n int, err error) {
	return conn.stream.Read(b)
}

func (conn *sessConn) Write(b []byte) (n int, err error) {
	return conn.stream.Write(b)
}

func (conn *sessConn) CloseWrite() error {
	return conn.stream.Close()
}

func (conn *sessConn) Close() error {
	err := conn.stream.Close()
	if err != nil {
		return err
	}

	err = conn.sess.CloseWithError(quic.ApplicationErrorCode(0), "")
	if err != nil {
		return err
	}
	return nil
}

// DoListenQUIC listens on a QUIC socket
func DoListenQUIC(port uint16) (chan io.ReadWriteCloser, error) {
	listener, err := appquic.ListenPort(
		port,
		&tls.Config{
			Certificates: appquic.GetDummyTLSCerts(),
			NextProtos:   nextProtos,
		},
		&quic.Config{KeepAlive: true},
	)
	if err != nil {
		return nil, err
	}

	conns := make(chan io.ReadWriteCloser)
	go func() {
		for {
			sess, err := listener.Accept(context.Background())
			if err != nil {
				logError("Can't accept listener", "err", err)
				continue
			}

			stream, err := sess.AcceptStream(context.Background())
			if err != nil {
				logError("Can't accept stream", "err", err)
				continue
			}

			conns <- &sessConn{
				sess:   sess,
				stream: stream,
			}
		}
	}()

	return conns, nil
}

// DoDialQUIC dials with a QUIC socket
func DoDialQUIC(remoteAddr string) (io.ReadWriteCloser, error) {
	sess, err := appquic.Dial(
		remoteAddr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         nextProtos,
		},
		&quic.Config{KeepAlive: true},
	)
	if err != nil {
		return nil, err
	}

	stream, err := sess.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}

	return &sessConn{
		sess:   sess,
		stream: stream,
	}, nil
}
