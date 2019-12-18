// Copyright 2019 ETH Zurich
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

// appquic provides a simple interface to use QUIC over SCION.
// This package is similar to snet/squic, but offers a smoother interface for
// applications and, like appnet, it allows to Dial hostnames resolved with RAINS.
package appquic

import (
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/lucas-clemente/quic-go"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	// Don't verify the server's cert, as we are not using the TLS PKI.
	cliTlsCfg        = &tls.Config{InsecureSkipVerify: true}
	srvTlsCfg        *tls.Config
	srvTlsCfgInit    sync.Once
	srvTlsCfgInitErr error
)

// closerSession is a wrapper around quic.Session that always closes the
// underlying sconn when closing the session.
// This is needed here because we use quic.Dial, not quic.DialAddr but we want
// the close-the-socket behaviour of quic.DialAddr.
type closerSession struct {
	quic.Session
	conn snet.Conn
}

func (s *closerSession) Close() error {
	s.conn.Close()
	return s.Session.Close()
}

func Dial(remote string, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Session, error) {
	remoteAddr, err := appnet.ResolveUDPAddr(remote)
	if err != nil {
		return nil, err
	}
	sconn, err := appnet.Listen(nil)
	if err != nil {
		return nil, err
	}
	if tlsConf == nil {
		tlsConf = cliTlsCfg
	}
	session, err := quic.Dial(sconn, remoteAddr, "host:0", tlsConf, quicConfig)
	if err != nil {
		return nil, err
	}
	return &closerSession{session, sconn}, nil
}

func ListenPort(port uint16, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {
	sconn, err := appnet.ListenPort(port)
	if err != nil {
		return nil, err
	}
	if tlsConf == nil {
		tlsConf, err = GetDummyTLSConfig()
		if err != nil {
			return nil, err
		}
	}
	return quic.Listen(sconn, tlsConf, quicConfig)
}

// GetDummyTLSConfig returns the (singleton) default server TLS config with a fresh
// private key and a dummy certificate.
func GetDummyTLSConfig() (*tls.Config, error) {
	srvTlsCfgInit.Do(func() {
		cert, err := generateKeyAndCert()
		if err != nil {
			srvTlsCfgInitErr = fmt.Errorf("appquic: Unable to generate dummy TLS cert/key: %v", err)
		}
		srvTlsCfg = &tls.Config{Certificates: []tls.Certificate{*cert}}
	})
	return srvTlsCfg, srvTlsCfgInitErr
}
