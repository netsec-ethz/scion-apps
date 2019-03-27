// Copyright 2018 ETH Zurich
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
// limitations under the License.package main

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	libaddr "github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
	// "github.com/scionproto/scion/go/lib/snet/squic"
)

var (
	// cliTlsCfg is a copy squic.cliTlsCfg
	cliTlsCfg = &tls.Config{InsecureSkipVerify: true}
)

// Transport wraps a h2quic.RoundTripper making it compatible with SCION
type Transport struct {
	LAddr              *snet.Addr
	QuicConfig         *quic.Config
	DisableCompression bool

	rt *h2quic.RoundTripper

	dialOnce         sync.Once
	connectionsMutex sync.Mutex
	connections      []*snet.Conn
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	return t.RoundTripOpt(req, h2quic.RoundTripOpt{})
}

// squicDialSCION re-implements squic.DialSCION to get access to the underlying
// snet.Conn to be able to close the connection later.
func (t *Transport) squicDialSCION(network *snet.SCIONNetwork, laddr, raddr *snet.Addr,
	quicConfig *quic.Config) (quic.Session, error) {

	// squic.sListen (but without the Bind/SVC)
	if network == nil {
		network = snet.DefNetwork
	}
	sconn, err := network.ListenSCION("udp4", laddr, 0)
	if err != nil {
		return nil, err
	}

	t.connectionsMutex.Lock()
	defer t.connectionsMutex.Unlock()
	t.connections = append(t.connections, &sconn)

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	return quic.Dial(sconn, raddr, "host:0", cliTlsCfg, quicConfig)
}

// RoundTripOpt is the same as RoundTrip but takes additional options
func (t *Transport) RoundTripOpt(req *http.Request, opt h2quic.RoundTripOpt) (*http.Response, error) {

	// initialize the SCION networking context once for all Transports
	initOnce.Do(func() {
		if snet.DefNetwork == nil {
			initErr = scionutil.InitSCION(t.LAddr)
		}
	})
	if initErr != nil {
		return nil, initErr
	}

	// set the dial function and QuicConfig once for each Transport
	t.dialOnce.Do(func() {
		dial := func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {

			/* TODO(chaehni):
			RequestConnectionIDOmission MUST not be set to 'true' when a connection is dialed using an existing net.PacketConn
			(which the squic package is doing)
			quic-go/client.go func populateClientConfig (line 177) fails to catch this problem
			As a result, if set to 'true' the quic-go client multiplexer fails to match incoming packets
			This problem is solved in subsequent releases of quic-go (>v0.8.0)
			See issue https://github.com/scionproto/scion/issues/2463
			*/
			cfg.RequestConnectionIDOmission = false

			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ia, l3, err := scionutil.GetHostByName(host)
			if err != nil {
				return nil, err
			}
			p, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				p = 443
			}
			l4 := libaddr.NewL4UDPInfo(uint16(p))
			raddr := &snet.Addr{IA: ia, Host: &libaddr.AppAddr{L3: l3, L4: l4}}
			return t.squicDialSCION(nil, t.LAddr, raddr, cfg)
		}
		t.rt = &h2quic.RoundTripper{
			Dial:               dial,
			QuicConfig:         t.QuicConfig,
			DisableCompression: t.DisableCompression,
		}
	})

	return t.rt.RoundTripOpt(req, opt)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *Transport) Close() error {
	err := t.rt.Close()

	// quic.Session.Close (which is called by RoundTripper.Close()) will NOT
	// close the underlying connections, so we do it manually here.
	t.connectionsMutex.Lock()
	defer t.connectionsMutex.Unlock()
	for _, sconn := range t.connections {
		(*sconn).Close()
	}

	return err
}
