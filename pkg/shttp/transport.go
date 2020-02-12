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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/scionproto/scion/go/lib/snet"
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
		&h2quic.RoundTripper{
			Dial:            dial,
			QuicConfig:      quicCfg,
			TLSClientConfig: tlsClientCfg,
		},
	}
}

var _ RoundTripper = (*roundTripper)(nil)

// roundTripper implements the RoundTripper interface. It wraps a
// h2quic.RoundTripper, making it compatible with SCION
type roundTripper struct {
	rt *h2quic.RoundTripper
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// If req.URL.Host is a SCION address, we need to mangle it so it passes through
	// h2quic without tripping up.
	// Note: when using the http.Client, the URL must already arrive mangled
	// here, otherwise it would not have parsed.
	cpy := *req
	cpy.URL = new(url.URL)
	*cpy.URL = *req.URL
	cpy.URL.Host = mangleSCIONAddr(req.URL.Host)

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
func dial(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {
	return appquic.Dial(unmangleSCIONAddr(address), tlsCfg, cfg)
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
	return schemePart + userInfoPart + mangleSCIONAddr(hostPart) + tail
}

// mangleSCIONAddr mangles a SCION address string (if it is one) so it can be
// safely used in the host part of a URL.
func mangleSCIONAddr(address string) string {

	raddr, err := snet.UDPAddrFromString(address)
	if err != nil {
		return address
	}

	// Turn this into [IA,IP]:port format. This is a valid host in a URI, as per
	// the "IP-literal" case in RFC 3986, §3.2.2.
	// Unfortunately, this is not currently compatible with snet.UDPAddrFromString,
	// so this will have to be _unmangled_ before use.
	mangledAddr := fmt.Sprintf("[%s,%s]", raddr.IA, raddr.Host.IP)
	if raddr.Host.Port != 0 {
		mangledAddr += fmt.Sprintf(":%d", raddr.Host.Port)
	}
	return mangledAddr
}

// unmangleSCIONAddr returns a SCION address that can be parsed with
// with snet.UDPAddrFromString.
// If the input is not a SCION address (e.g. a hostname), the address is
// returned unchanged.
// This parses the address, so that it can safely join host and port, with the
// brackets in the right place. Yes, this means this will be parsed twice.
//
// Assumes that address always has a port (this is enforced by the h2quic
// roundtripper code)
func unmangleSCIONAddr(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		panic(fmt.Sprintf("unmangleSCIONAddr assumes that address is of the form host:port %s", err))
	}
	// brackets are removed from [I-A,IP] part by SplitHostPort, so this can be
	// parsed with UDPAddrFromString:
	udpAddr, err := snet.UDPAddrFromString(host)
	if err != nil {
		return address
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return address
	}
	udpAddr.Host.Port = int(p)
	return udpAddr.String()
}
