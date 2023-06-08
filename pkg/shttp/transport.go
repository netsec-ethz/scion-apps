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

// package shttp provides glue to use net/http libraries for HTTP over SCION.
package shttp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/quic-go/quic-go"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

// DefaultTransport is the default RoundTripper that can be used for HTTP over
// SCION/QUIC.
// This is equivalent to net/http.DefaultTransport with DialContext overridden
// to use shttp.Dialer, which dials connections over SCION/QUIC.
var DefaultTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&Dialer{
		QuicConfig: nil,
		Policy:     nil,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// NewTransport creates a new RoundTripper that can be used for HTTP over
// SCION/QUIC, providing a custom QuicConfig and Policy to the Dialer.
// This equivalent to net/http.DefaultTransport with an overridden DialContext.
// Both the Transport and the Dialer are returned, as the Dialer is not otherwise
// accessible from the Transport.
func NewTransport(quicCfg *quic.Config, policy pan.Policy) (*http.Transport, *Dialer) {
	dialer := &Dialer{
		QuicConfig: quicCfg,
		Policy:     policy,
	}
	transport := DefaultTransport.Clone()
	transport.DialContext = dialer.DialContext
	return transport, dialer
}

// Dialer dials an insecure, single-stream QUIC connection over SCION (just pretend it's TCP).
// This is the Dialer used for shttp.DefaultTransport.
type Dialer struct {
	Local      netaddr.IPPort
	QuicConfig *quic.Config
	Policy     pan.Policy
	sessions   []*pan.QUICSession
}

// DialContext dials an insecure, single-stream QUIC connection over SCION. This can be used
// as the DialContext function in net/http.Transport.
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	tlsCfg := &tls.Config{
		NextProtos:         []string{quicutil.SingleStreamProto},
		InsecureSkipVerify: true,
	}

	remote, err := pan.ResolveUDPAddr(ctx, pan.UnmangleSCIONAddr(addr))
	if err != nil {
		return nil, err
	}

	session, err := pan.DialQUIC(ctx, d.Local, remote, d.Policy, nil, addr, tlsCfg, d.QuicConfig)
	if err != nil {
		return nil, err
	}
	d.sessions = append(d.sessions, session)
	return quicutil.NewSingleStream(session)
}

func (d *Dialer) SetPolicy(policy pan.Policy) {
	d.Policy = policy
	for _, s := range d.sessions {
		s.Conn.SetPolicy(policy)
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
	return schemePart + userInfoPart + pan.MangleSCIONAddr(hostPart) + tail
}
