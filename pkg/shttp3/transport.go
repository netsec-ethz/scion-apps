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
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

// DefaultTransport is the default RoundTripper that can be used for HTTP/3
// over SCION.
var DefaultTransport = &http3.RoundTripper{
	Dial: (&Dialer{
		Policy: nil,
	}).Dial,
}

// Dialer dials a QUIC connection over SCION.
// This is the Dialer used for shttp3.DefaultTransport.
type Dialer struct {
	Local    netaddr.IPPort
	Policy   pan.Policy
	sessions []*pan.QUICEarlySession
}

// Dial dials a QUIC connection over SCION.
func (d *Dialer) Dial(network, addr string, tlsCfg *tls.Config,
	cfg *quic.Config) (quic.EarlySession, error) {

	remote, err := pan.ResolveUDPAddr(context.TODO(), pan.UnmangleSCIONAddr(addr))
	if err != nil {
		return nil, err
	}
	session, err := pan.DialQUICEarly(context.TODO(), d.Local, remote, d.Policy, nil, addr, tlsCfg, cfg)
	if err != nil {
		return nil, err
	}
	d.sessions = append(d.sessions, session)
	return session, nil
}

func (d *Dialer) SetPolicy(policy pan.Policy) {
	d.Policy = policy
	for _, s := range d.sessions {
		s.Conn.SetPolicy(policy)
	}
}
