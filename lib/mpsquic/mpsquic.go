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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/tools/scmp/cmn"
)

const (
	defKeyPath = "gen-certs/tls.key"
	defPemPath = "gen-certs/tls.pem"
)

type MPQuic struct {
	SCIONFlexConnection *SCIONFlexConn
	raddrs              []*snet.Addr
	Qsession            quic.Session
	dispConn            *reliable.Conn
}

var (
	// Don't verify the server's cert, as we are not using the TLS PKI.
	cliTlsCfg = &tls.Config{InsecureSkipVerify: true}
	srvTlsCfg = &tls.Config{}
	noDisp = false
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

func (mpq *MPQuic) Close() error {
	if !noDisp && mpq.dispConn != nil {
		err := mpq.dispConn.Close()
		if err != nil {
			return  err
		}
	}
	return mpq.SCIONFlexConnection.Close()
}

func (mpq *MPQuic) Monitor() error {
	if noDisp {
		return nil
	}
	for {
		id := cmn.Rand()
		info := &scmp.InfoEcho{Id: id, Seq: 0}
		pkt := cmn.NewSCMPPkt(scmp.T_G_EchoRequest, info, nil)
		b := make(common.RawBytes, cmn.Mtu)
		nhAddr := cmn.NextHopAddr()

		nextPktTS := time.Now()


		cmn.UpdatePktTS(pkt, nextPktTS)
		// Serialize packet to internal buffer
		pktLen, err := hpkt.WriteScnPkt(pkt, b)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to serialize SCION packet %v\n", err)
			break
		}
		written, err := mpq.dispConn.WriteTo(b[:pktLen], nhAddr)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to write %v\n", err)
			break
		} else if written != pktLen {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Wrote incomplete message. written=%d, expected=%d\n",
				len(b), written)
			break
		}
		cmn.Stats.Sent += 1
		// More packets?
		if cmn.Count != 0 && cmn.Stats.Sent == cmn.Count {
			break
		}
		// Update packet fields
		info.Seq += 1
		payload := pkt.Pld.(common.RawBytes)
		_, _ = info.Write(payload[scmp.MetaLen:])


		if true {
			break
		}
	}

	for {


		pkt := &spkt.ScnPkt{}
		b := make(common.RawBytes, cmn.Mtu)

		pktLen, err := mpq.dispConn.Read(b)
		if err != nil {
			if common.IsTimeoutErr(err) {
				continue
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: Unable to read: %v\n", err)
				break
			}
		}
		now := time.Now()
		err = hpkt.ParseScnPkt(pkt, b[:pktLen])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: SCION packet parse: %v\n", err)
			continue
		}
		// Validate packet
		var scmpHdr *scmp.Hdr
		var info *scmp.InfoEcho

		// XXX: Check the InfoEcho.ID to disambiguate between the connections

		cmn.Stats.Recv += 1

		// Calculate return time
		rtt := now.Sub(scmpHdr.Time()).Round(time.Microsecond)
		fmt.Println(pkt, pktLen, info, rtt)


		if true {
			break
		}
	}
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

	// Connect to the dispatcher
	var overlayBindAddr *overlay.OverlayAddr
	if baddr != nil {
		if baddr.Host != nil {
			overlayBindAddr, err = overlay.NewOverlayAddr(baddr.Host.L3, baddr.Host.L4)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Failed to create bind address: %v\n", err))
			}
		}
	}
	dispConn, _, err := reliable.Register(reliable.DefaultDispPath, laddr.IA, laddr.Host,
		overlayBindAddr, addr.SvcNone)
	if err != nil {
		//return nil, errors.New(fmt.Sprintf("Unable to register with the dispatcher addr=%s\nerr=%v", laddr, err))
		noDisp = true
	}

	flexConn := newSCIONFlexConn(sconn, raddrs[0])

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}
	return &MPQuic{SCIONFlexConnection: flexConn, Qsession: qsession, dispConn: dispConn, raddrs: raddrs}, nil
}

// This switches between different SCION paths as given by the SCION address with path structs in raddrs
func SwitchMPConn(mpConn *MPQuic) (*MPQuic, error) {
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
