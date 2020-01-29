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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/scionproto/scion/go/lib/addr"
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
func dial(network, addrStr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {

	host, port, err := net.SplitHostPort(addrStr)
	if err != nil {
		return nil, err
	}
	if isMangledSCIONAddr(host) {
		host, err = unmangleSCIONAddr(host)
		if err != nil {
			return nil, err
		}
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		p = 443
	}
	return appquic.Dial(fmt.Sprintf("%s:%d", host, p), tlsCfg, cfg)
}

var scionAddrURLRegexp = regexp.MustCompile(
	`^(\w*://)?(\w+@)?(\d+-[\d:A-Fa-f]+,\[[^\]]+\])(.*)$`)

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
	scionAddrPart := match[3]
	tail := match[4]
	return schemePart + userInfoPart + mangleSCIONAddr(scionAddrPart) + tail
}

// mangleSCIONAddr mangles a SCION address string (if it is one) so it can be
// safely used in the host part of a URL.
func mangleSCIONAddr(address string) string {

	raddr, err := snet.UDPAddrFromString(address)
	if err != nil {
		return address
	}
	// The Host will be either IPv4 or IPv6.
	// To make this a valid host string for a URL, replace : for IPv6 addresses by ~. There will
	// not be any other tildes, so no need to escape them.
	host := raddr.Host.String()
	hostMangled := strings.Replace(host, ":", "~", -1)

	mangledAddr := fmt.Sprintf("__%s__%s__", raddr.IA.FileFmt(false), hostMangled)
	if raddr.Host.Port != 0 {
		mangledAddr += fmt.Sprintf(":%d", raddr.Host.Port)
	}
	return mangledAddr
}

// isMangledSCIONAddr checks if this is an address previously encoded with mangleSCIONAddr
// without port, *after* SplitHostPort has been applied.
func isMangledSCIONAddr(host string) bool {

	parts := strings.Split(host, "__")
	return len(parts) == 4 && len(parts[0]) == 0 && len(parts[3]) == 0
}

// unmangleSCIONAddr decodes and parses a SCION-address previously encoded with mangleSCIONAddr
// without port, i.e. *after* SplitHostPort has been applied.
func unmangleSCIONAddr(host string) (string, error) {

	parts := strings.Split(host, "__")
	ia, err := addr.IAFromFileFmt(parts[1], false)
	if err != nil {
		return "", err
	}
	ipStr := strings.Replace(parts[2], "~", ":", -1)
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", errors.New("could not parse IP in SCION-address")
	}
	return fmt.Sprintf("%s,[%v]", ia, ip), nil
}
