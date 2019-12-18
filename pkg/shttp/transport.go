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
	"strconv"
	"strings"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

type Transport interface {
	http.RoundTripper
	io.Closer
}

func NewTransport(tlsClientCfg *tls.Config, quicCfg *quic.Config) Transport {
	return &transport{
		&h2quic.RoundTripper{
			Dial:            dial,
			QuicConfig:      quicCfg,
			TLSClientConfig: tlsClientCfg,
		},
	}
}

var _ Transport = (*transport)(nil)

// Transport wraps a h2quic.RoundTripper making it compatible with SCION
type transport struct {
	rt *h2quic.RoundTripper
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {

	// If req.URL.Host is a SCION address, we need to mangle it so it passes through
	// h2quic without tripping up.
	mangled_host := mangleSCIONAddr(req.URL.Host)
	cpy := *req
	cpy.URL = new(url.URL)
	*cpy.URL = *req.URL
	cpy.URL.Host = mangled_host

	return t.rt.RoundTrip(&cpy)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *transport) Close() (err error) {

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

// mangleSCIONAddr encodes the given SCION address so that it can be safely
// used in the host part of a URL.
func mangleSCIONAddr(address string) string {

	raddr, err := snet.AddrFromString(address)
	if err != nil {
		return address // if it doesn't parse, it's probably not a SCION address.
	}
	// The HostAddr will be either IPv4 or IPv6 (not a HostSVC-addr).
	// To make this a valid host string for a URL, replace : for IPv6 addresses by ~. There will
	// not be any other tildes, so no need to escape them.
	l3 := raddr.Host.L3.String()
	l3_mangled := strings.Replace(l3, ":", "~", -1)

	u := fmt.Sprintf("__%s__%s__", raddr.IA.FileFmt(false), l3_mangled)
	if raddr.Host.L4 != 0 {
		u += fmt.Sprintf(":%d", raddr.Host.L4)
	}
	return u
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
	l3_str := strings.Replace(parts[2], "~", ":", -1)
	l3 := addr.HostFromIPStr(l3_str)
	if l3 == nil {
		return "", errors.New("Could not parse IP in SCION-address")
	}
	h := &snet.Addr{IA: ia, Host: &addr.AppAddr{L3: l3}}
	return h.String(), nil
}
