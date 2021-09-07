// Copyright 2021 ETH Zurich
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

// package shttp3 provides glue to use quic-go/http3 libraries for HTTP/3 over
// SCION.
package shttp3

import (
	"crypto/tls"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

// DefaultTransport is the default RoundTripper that can be used for HTTP/3
// over SCION.
var DefaultTransport = &http3.RoundTripper{
	Dial: Dial,
}

// Dial is the Dial function used in RoundTripper
func Dial(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error) {
	return appquic.DialEarly(appnet.UnmangleSCIONAddr(address), tlsCfg, cfg)
}
