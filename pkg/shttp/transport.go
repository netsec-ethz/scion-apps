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

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

// DefaultTransport is the default RoundTripper that can be used for HTTP over
// SCION/QUIC.
// This is equivalent to net/http.DefaultTransport with DialContext overridden
// to use shttp.Dialer, which dials connections over SCION/QUIC.
var DefaultTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&Dialer{
		QuicConfig: nil,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// Dialer dials an insecure, single-stream QUIC connection over SCION (just pretend it's TCP).
// This is the Dialer used for shttp.DefaultTransport.
type Dialer struct {
	QuicConfig *quic.Config
}

// DialContext dials an insecure, single-stream QUIC connection over SCION. This can be used
// as the DialContext function in net/http.Transport.
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	tlsCfg := &tls.Config{
		NextProtos:         []string{nextProtoRaw},
		InsecureSkipVerify: true,
	}
	sess, err := appquic.Dial(appnet.UnmangleSCIONAddr(addr), tlsCfg, d.QuicConfig)
	if err != nil {
		return nil, err
	}
	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return singleStreamSession{sess, stream}, nil
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
