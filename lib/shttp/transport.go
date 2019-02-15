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

	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	libaddr "github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

// Transport wraps a h2quic.RoundTripper and makes it compatible with SCION
type Transport struct {
	LAddr *snet.Addr

	rt *h2quic.RoundTripper

	dialOnce sync.Once
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	return t.RoundTripOpt(req, h2quic.RoundTripOpt{})
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

	// set the dial function once for each Transport
	t.dialOnce.Do(func() {
		dial := func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			raddr, err := scionutil.GetHostByName(host)
			if err != nil {
				return nil, err
			}
			p, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				p = 443
			}
			raddr.Host.L4 = libaddr.NewL4UDPInfo(uint16(p))
			return squic.DialSCION(nil, t.LAddr, raddr, nil)
		}
		t.rt = &h2quic.RoundTripper{
			Dial: dial,
		}
	})

	return t.rt.RoundTripOpt(req, opt)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *Transport) Close() error {
	return t.rt.Close()
}
