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

// Package appquic provides a simple interface to use QUIC over SCION.
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
	cliTLSCfg     = &tls.Config{InsecureSkipVerify: true}
	srvTLSCfg     *tls.Config
	srvTLSCfgInit sync.Once
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

// Dial establishes a new QUIC connection to a server at the remote address.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of hostname:port.
func Dial(remote string, tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, error) {
	raddr, err := appnet.ResolveUDPAddr(remote)
	if err != nil {
		return nil, err
	}
	return DialAddr(raddr, tlsConf, quicConf)
}

// DialAddr establishes a new QUIC connection to a server at the remote address.
//
// If no path is specified in raddr, DialAddr will choose the first available path,
// analogous to appnet.DialAddr.
func DialAddr(raddr *snet.UDPAddr, tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, error) {
	if raddr.Path == nil {
		err := appnet.SetDefaultPath(raddr)
		if err != nil {
			return nil, err
		}
	}
	sconn, err := appnet.Listen(nil)
	if err != nil {
		return nil, err
	}
	if tlsConf == nil {
		tlsConf = cliTLSCfg
	}
	session, err := quic.Dial(sconn, raddr, "host:0", tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &closerSession{session, sconn}, nil
}

// ListenPort listens for QUIC connections on a SCION/UDP port.
//
// See note on wildcard addresses in the appnet package documentation.
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
	var initErr error
	srvTLSCfgInit.Do(func() {
		cert, err := generateKeyAndCert()
		if err != nil {
			initErr = fmt.Errorf("appquic: Unable to generate dummy TLS cert/key: %v", err)
		}
		srvTLSCfg = &tls.Config{Certificates: []tls.Certificate{*cert}}
	})
	return srvTLSCfg, initErr
}
