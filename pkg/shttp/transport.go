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
// limitations under the License.

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"regexp"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

// RoundTripper implements the RoundTripper interface. It wraps a
// http3.RoundTripper to make connections over SCION.
type RoundTripper struct {
	rt       *http3.RoundTripper
	policy   pan.Policy
	sessions []*pan.QUICEarlySession
}

// dialFunc is the function type supported in http3.RoundTripper.Dial
type dialFunc func(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error)

// NewRoundTripper creates a new RoundTripper that can be used as the Transport
// of an http.Client.
func NewRoundTripper(policy pan.Policy, tlsClientCfg *tls.Config, quicCfg *quic.Config) *RoundTripper {
	t := &RoundTripper{
		rt: &http3.RoundTripper{
			QuicConfig:      quicCfg,
			TLSClientConfig: tlsClientCfg,
		},
		policy: policy,
	}
	t.rt.Dial = t.dialer()
	return t
}

// RoundTrip does a single round trip; retrieving a response for a given request
func (t *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// If req.URL.Host is a SCION address, we need to mangle it so it passes through
	// http3 without tripping up.
	// Note: when using the http.Client, the URL must already arrive mangled
	// here, otherwise it would not have parsed.
	cpy := *req
	cpy.URL = new(url.URL)
	*cpy.URL = *req.URL
	cpy.URL.Host = appnet.MangleSCIONAddr(req.URL.Host)

	return t.rt.RoundTrip(&cpy)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *RoundTripper) Close() (err error) {
	if t.rt != nil {
		err = t.rt.Close()
	}
	return err
}

// dialer creates a `Dial` function to be used in http3.RoundTrip.Dial,
// capturing the *RoundTripper.
func (t *RoundTripper) dialer() dialFunc {
	// dial is the Dial function used in RoundTripper
	return func(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error) {
		hostname := address
		// TODO: support hostnames in pan
		// XXX: roundtrip through string representation, parse twice.
		addrResolved, err := appnet.ResolveUDPAddr(appnet.UnmangleSCIONAddr(address))
		if err != nil {
			return nil, err
		}
		addr, err := pan.ParseUDPAddr(addrResolved.String())
		if err != nil {
			panic("parse error after already parsing successfully once, should not happen")
		}
		session, err := pan.DialQUICEarly(context.Background(),
			nil, addr, t.policy, nil,
			hostname, tlsCfg, cfg)
		if err != nil {
			return nil, err
		}
		t.sessions = append(t.sessions, session)
		return session, err
	}
}

func (t *RoundTripper) SetPolicy(policy pan.Policy) {
	t.policy = policy
	for _, s := range t.sessions {
		s.SetPolicy(policy)
	}
}

var scionAddrURLRegexp = regexp.MustCompile(
	`^(\w*://)?(\w+@)?([^/?]*)(.*)$`)

// MangleSCIONAddrURL mangles a SCION address in the host part of a URL-ish
// string so that it can be safely used as a URL, i.e. it can be parsed by
// net/url.Parse
func MangleSCIONAddrURL(url string) string {
	match := scionAddrURLRegexp.FindStringSubmatch(url)
	if len(match) == 0 {
		return url // does not match: it's not a URL or not a URL with a SCION address. Just pass it through.
	}

	schemePart := match[1]
	userInfoPart := match[2]
	hostPart := match[3]
	tail := match[4]
	return schemePart + userInfoPart + appnet.MangleSCIONAddr(hostPart) + tail
}
