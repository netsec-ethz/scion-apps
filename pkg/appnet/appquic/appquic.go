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
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/lucas-clemente/quic-go"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	srvTLSDummyCerts     []tls.Certificate
	srvTLSDummyCertsInit sync.Once
)

// closerSession is a wrapper around quic.Session that always closes the
// underlying sconn when closing the session.
// This is needed here because we use quic.Dial, not quic.DialAddr but we want
// the close-the-socket behaviour of quic.DialAddr.
type closerSession struct {
	quic.Session
	conn *snet.Conn
}

func (s *closerSession) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	s.conn.Close()
	return s.Session.CloseWithError(code, desc)
}

// closerEarlySession is a wrapper around quic.EarlySession, analogous to closerSession
type closerEarlySession struct {
	quic.EarlySession
	conn *snet.Conn
}

func (s *closerEarlySession) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	s.conn.Close()
	return s.EarlySession.CloseWithError(code, desc)
}

// Dial establishes a new QUIC connection to a server at the remote address.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of hostname:port.
func Dial(remote string, tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, error) {
	raddr, err := appnet.ResolveUDPAddr(remote)
	if err != nil {
		return nil, err
	}
	return DialAddr(raddr, remote, tlsConf, quicConf)
}

// DialAddr establishes a new QUIC connection to a server at the remote address.
//
// If no path is specified in raddr, DialAddr will choose the first available path,
// analogous to appnet.DialAddr.
// The host parameter is used for SNI.
// The tls.Config must define an application protocol (using NextProtos).
func DialAddr(raddr *snet.UDPAddr, host string, tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, error) {
	err := ensurePathDefined(raddr)
	if err != nil {
		return nil, err
	}
	sconn, err := appnet.Listen(nil)
	if err != nil {
		return nil, err
	}
	host = appnet.MangleSCIONAddr(host)
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	session, err := quic.Dial(sconn, raddr, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	return &closerSession{session, sconn}, nil
}

// DialEarly establishes a new 0-RTT QUIC connection to a server. Analogous to Dial.
func DialEarly(remote string, tlsConf *tls.Config, quicConf *quic.Config) (quic.EarlySession, error) {
	raddr, err := appnet.ResolveUDPAddr(remote)
	if err != nil {
		return nil, err
	}
	return DialAddrEarly(raddr, remote, tlsConf, quicConf)
}

// DialAddrEarly establishes a new 0-RTT QUIC connection to a server. Analogous to DialAddr.
func DialAddrEarly(raddr *snet.UDPAddr, host string, tlsConf *tls.Config, quicConf *quic.Config) (quic.EarlySession, error) {
	err := ensurePathDefined(raddr)
	if err != nil {
		return nil, err
	}
	sconn, err := appnet.Listen(nil)
	if err != nil {
		return nil, err
	}
	host = appnet.MangleSCIONAddr(host)
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	session, err := quic.DialEarly(sconn, raddr, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	// XXX(matzf): quic.DialEarly seems to have the wrong return type declared (quic.DialAddrEarly returns EarlySession)
	return &closerEarlySession{session.(quic.EarlySession), sconn}, nil
}

func ensurePathDefined(raddr *snet.UDPAddr) error {
	if raddr.Path.IsEmpty() {
		return appnet.SetDefaultPath(raddr)
	}
	return nil
}

// ListenPort listens for QUIC connections on a SCION/UDP port.
//
// See note on wildcard addresses in the appnet package documentation.
func ListenPort(port uint16, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {
	return Listen(&net.UDPAddr{Port: int(port)}, tlsConf, quicConfig)
}

func Listen(listen *net.UDPAddr, tlsConf *tls.Config, quicConfig *quic.Config) (quic.Listener, error) {
	sconn, err := appnet.Listen(listen)
	if err != nil {
		return nil, err
	}
	// HACK: we silence the log here to shut up quic-go's warning about trying to
	// set receive buffer size (it's not a UDPConn, we know).
	silenceLog()
	defer unsilenceLog()
	return quic.Listen(sconn, tlsConf, quicConfig)
}

// GetDummyTLSCert returns the singleton TLS certificate with a fresh
// private key and a dummy certificate.
func GetDummyTLSCerts() []tls.Certificate {
	var initErr error
	srvTLSDummyCertsInit.Do(func() {
		cert, err := generateKeyAndCert()
		if err != nil {
			initErr = fmt.Errorf("appquic: Unable to generate dummy TLS cert/key: %v", err)
		}
		srvTLSDummyCerts = []tls.Certificate{*cert}
	})
	if initErr != nil {
		panic(initErr)
	}
	return srvTLSDummyCerts
}

var logSilencerCount int32
var logSilencerOriginal io.Writer

// silenceLog redirects the log.Default writer to a black hole.
// It can be reenabled by calling unsilenceLog.
// These functions can safely be called from multiple goroutines concurrently;
// the log will remain silenced until unsilenceLog was called for each
// silenceLog call.
func silenceLog() {
	count := atomic.AddInt32(&logSilencerCount, 1)
	if count == 1 {
		logSilencerOriginal = log.Default().Writer()
		log.Default().SetOutput(blackhole{})
	}
}

func unsilenceLog() {
	count := atomic.AddInt32(&logSilencerCount, -1)
	if count == 0 {
		log.Default().SetOutput(logSilencerOriginal)
		logSilencerOriginal = nil
	}
}

type blackhole struct{}

func (w blackhole) Write(p []byte) (n int, err error) {
	return len(p), nil
}
