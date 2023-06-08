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
	"time"

	"github.com/quic-go/quic-go"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

var (
	nextProtos = []string{quicutil.SingleStreamProto}
)

// DoListenQUIC listens on a QUIC socket
func DoListenQUIC(port uint16) (chan io.ReadWriteCloser, error) {
	quicListener, err := pan.ListenQUIC(
		context.Background(),
		netaddr.IPPortFrom(netaddr.IP{}, port),
		nil,
		&tls.Config{
			Certificates: quicutil.MustGenerateSelfSignedCert(),
			NextProtos:   nextProtos,
		},
		&quic.Config{KeepAlivePeriod: time.Duration(15) * time.Second},
	)
	if err != nil {
		return nil, err
	}
	listener := quicutil.SingleStreamListener{Listener: quicListener}

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
	remoteAddr, err := pan.ResolveUDPAddr(context.TODO(), remote)
	if err != nil {
		return nil, err
	}
	sess, err := pan.DialQUIC(
		context.Background(),
		netaddr.IPPort{},
		remoteAddr,
		policy,
		nil,
		pan.MangleSCIONAddr(remote),
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         nextProtos,
		},
		&quic.Config{KeepAlivePeriod: time.Duration(15) * time.Second},
	)
	if err != nil {
		return nil, err
	}

	return quicutil.NewSingleStream(sess)
}
