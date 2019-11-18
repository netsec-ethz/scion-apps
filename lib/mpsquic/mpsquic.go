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
	active              int
	Qsession            quic.Session
	dispConn            *reliable.Conn
	raddrRTTs           []time.Duration
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
		tmp := mpq.dispConn
		mpq.dispConn = nil
		time.Sleep(time.Second)
		err := tmp.Close()
		if err != nil {
			return  err
		}
	}
	return mpq.SCIONFlexConnection.Close()
}

func (mpq *MPQuic) sendSCMP() {
	for {
		if mpq.dispConn == nil {
			break
		}

		for i := range mpq.raddrs {
			cmn.Remote = *mpq.raddrs[i]
			id := uint64(i+1)
			info := &scmp.InfoEcho{Id: id, Seq: 0}
			pkt := cmn.NewSCMPPkt(scmp.T_G_EchoRequest, info, nil)
			b := make(common.RawBytes, 1500) // TODO: Get proper MTU from PathEntry
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

			payload := pkt.Pld.(common.RawBytes)
			_, _ = info.Write(payload[scmp.MetaLen:])
			//fmt.Println("Sent SCMP packet, len:", pktLen, "payload", payload, "ID", info.Id)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (mpq *MPQuic) rcvSCMP() {
	for {
		if mpq.dispConn == nil {
			break
		}

		pkt := &spkt.ScnPkt{}
		b := make(common.RawBytes, 1500)

		pktLen, err := mpq.dispConn.Read(b)
		if err != nil {
			if common.IsTimeoutErr(err) {
				continue
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to read: %v\n", err)
				break
			}
		}
		now := time.Now()
		err = hpkt.ParseScnPkt(pkt, b[:pktLen])
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: SCION packet parse: %v\n", err)
			continue
		}
		// Validate scmp packet
		var scmpHdr *scmp.Hdr
		var scmpPld *scmp.Payload
		var info *scmp.InfoEcho

		scmpHdr, ok := pkt.L4.(*scmp.Hdr)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "Not an SCMP header", nil, "type", common.TypeOf(pkt.L4))
			continue
		}
		scmpPld, ok = pkt.Pld.(*scmp.Payload)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "Not an SCMP payload", nil, "type", common.TypeOf(pkt.Pld))
			continue
		}
		_ = scmpPld
		info, ok = scmpPld.Info.(*scmp.InfoEcho)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "Not an Info Echo", nil, "type", common.TypeOf(scmpPld.Info))
			continue
		}

		cmn.Stats.Recv += 1

		// Calculate RTT
		rtt := now.Sub(scmpHdr.Time()).Round(time.Microsecond)
		if info.Id - 1 < uint64(len(mpq.raddrRTTs)) {
			//fmt.Println("Received SCMP packet, len:", pktLen, "ID", info.Id)
			mpq.raddrRTTs[info.Id - 1] = rtt
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "Wrong InfoEcho Id", nil, "id", info.Id)
		}
	}
}

func (mpq *MPQuic) Monitor() error {
	cmn.Remote = *mpq.SCIONFlexConnection.raddr
	cmn.Local = *mpq.SCIONFlexConnection.laddr
	if cmn.Stats == nil {
		cmn.Stats = &cmn.ScmpStats{}
	}

	go mpq.sendSCMP()
	go mpq.rcvSCMP()

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
	laddrMonitor := laddr.Copy()
	laddrMonitor.Host.L4 = addr.NewL4UDPInfo(laddr.Host.L4.Port()+1)
	dispConn, _, err := reliable.Register(reliable.DefaultDispPath, laddrMonitor.IA, laddrMonitor.Host,
		overlayBindAddr, addr.SvcNone)
	if err != nil {
		//return nil, errors.New(fmt.Sprintf("Unable to register with the dispatcher addr=%s\nerr=%v", laddr, err))
		fmt.Printf("mpsquic: l. 199:\n%v\n", err)
		noDisp = true
	}

	active := 0
	flexConn := newSCIONFlexConn(sconn, laddr, raddrs[active])

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}

	raddrRTTs := []time.Duration{}
	for _ = range raddrs {
		raddrRTTs = append(raddrRTTs, 1<<63 - 1)
	}
	mpQuic := &MPQuic{SCIONFlexConnection: flexConn, raddrs: raddrs, active: active, Qsession: qsession, dispConn: dispConn, raddrRTTs: raddrRTTs}

	_ = mpQuic.Monitor()

	return mpQuic, nil
}

func (mpq *MPQuic) policyLowerRTTMatch(candidate int) bool {
	if mpq.raddrRTTs[candidate] < mpq.raddrRTTs[mpq.active] {
		return true
	}
	return false
}

// This switches between different SCION paths as given by the SCION address with path structs in raddrs
// The force flag makes switching a requirement, continuing to use the existing path is not an option
func SwitchMPConn(mpq *MPQuic, force bool) (*MPQuic, error) {
	// Display stats
	for i, rtt := range mpq.raddrRTTs {
		fmt.Printf("Measured RTT of %v on path %v\n", rtt, i)
	}

	// Right now, the QUIC session is returned unmodified
	// Still passing it in, since it might change later
	for i := range mpq.raddrs {
		if mpq.SCIONFlexConnection.raddr != mpq.raddrs[i] && mpq.policyLowerRTTMatch(i) {
			// fmt.Printf("Previous path: %v\n", mpConn.SCIONFlexConnection.raddr.Path)
			// fmt.Printf("New path: %v\n", mpConn.raddrs[i].Path)
			mpq.SCIONFlexConnection.SetRemoteAddr(mpq.raddrs[i])
			return mpq, nil
		}
	}
	if !force {
		return mpq, nil
	}

	return nil, common.NewBasicError("mpsquic: No fallback connection available.", nil)
}
