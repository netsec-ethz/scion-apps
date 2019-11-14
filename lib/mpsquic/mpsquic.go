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

// "Multiple paths" QUIC/SCION implementation.
package mpsquic

import (
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/snet"
)

const (
	defKeyPath = "gen-certs/tls.key"
	defPemPath = "gen-certs/tls.pem"
)

type MPQuic struct {
	SCIONFlexConnection *SCIONFlexConn
	raddrs              []*snet.Addr
	Qsession            quic.Session
}

var (
	// Don't verify the server's cert, as we are not using the TLS PKI.
	cliTlsCfg = &tls.Config{InsecureSkipVerify: true}
	srvTlsCfg = &tls.Config{}
)

func Init(keyPath, pemPath string) error {
	if keyPath == "" {
		keyPath = defKeyPath
	}
	if pemPath == "" {
		pemPath = defPemPath
	}
	cert, err := tls.LoadX509KeyPair(pemPath, keyPath)
	if err != nil {
		return common.NewBasicError("mpsquic: Unable to load TLS cert/key", err)
	}
	srvTlsCfg.Certificates = []tls.Certificate{cert}
	return nil
}

func DialMP(network *snet.SCIONNetwork, laddr *snet.Addr, raddrs []*snet.Addr,
	quicConfig *quic.Config) (*MPQuic, error) {

	return DialMPWithBindSVC(network, laddr, raddrs, nil, addr.SvcNone, quicConfig)
}

func DialMPWithBindSVC(network *snet.SCIONNetwork, laddr *snet.Addr, raddrs []*snet.Addr, baddr *snet.Addr,
	svc addr.HostSVC, quicConfig *quic.Config) (*MPQuic, error) {

	if network == nil {
		network = snet.DefNetwork
	}

	sconn, err := sListen(network, laddr, baddr, svc)

	flexConn := newSCIONFlexConn(sconn, raddrs[0])

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}
	return &MPQuic{SCIONFlexConnection: flexConn, raddrs: raddrs, Qsession: qsession}, nil
}

// This switches between different SCION paths as given by the SCION address with path structs in raddrs
func SwitchMPSCIONConn(mpConn *MPQuic) (*MPQuic, error) {
	// Right now, the QUIC session is returned unmodified
	// Still passing it in, since it might change later
	for i := range mpConn.raddrs {
		if mpConn.SCIONFlexConnection.raddr != mpConn.raddrs[i] {
			// fmt.Printf("Previous path: %v\n", mpConn.SCIONFlexConnection.raddr.Path)
			// fmt.Printf("New path: %v\n", mpConn.raddrs[i].Path)
			mpConn.SCIONFlexConnection.SetRemoteAddr(mpConn.raddrs[i])
			return mpConn, nil
		}
	}

	return nil, common.NewBasicError("mpsquic: No fallback connection available.", nil)
}
