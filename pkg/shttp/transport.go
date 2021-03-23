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

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

// NewRoundTripper is a convenience function that creates a new http.Transport
// with a SCION/QUIC Dialer.
func NewRoundTripper() *http.Transport {
	return &http.Transport{
		DialContext: (&Dialer{
			QuicConfig: nil,
		}).DialContext,
	}
}

// Dialer dials an insecure QUIC connection over SCION (just pretend it's TCP).
type Dialer struct {
	QuicConfig *quic.Config
}

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
