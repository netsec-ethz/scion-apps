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
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

// RoundTripper extends the http.RoundTripper interface with a Close
type RoundTripper interface {
	http.RoundTripper
	io.Closer
}

// NewRoundTripper creates a new RoundTripper that can be used as the Transport
// of an http.Client.
func NewRoundTripper(tlsClientCfg *tls.Config, quicCfg *quic.Config) RoundTripper {
	return &roundTripper{
		&http3.RoundTripper{
			Dial:            dial,
			QuicConfig:      quicCfg,
			TLSClientConfig: tlsClientCfg,
		},
	}
}

var _ RoundTripper = (*roundTripper)(nil)

// roundTripper implements the RoundTripper interface. It wraps a
// http3.RoundTripper, making it compatible with SCION
type roundTripper struct {
	rt *http3.RoundTripper
}

// RoundTrip does a single round trip; retrieving a response for a given request
func (t *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

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
func (t *roundTripper) Close() (err error) {

	if t.rt != nil {
		err = t.rt.Close()
	}

	return err
}

// dial is the Dial function used in RoundTripper
func dial(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error) {
	return appquic.DialEarly(appnet.UnmangleSCIONAddr(address), tlsCfg, cfg)
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
