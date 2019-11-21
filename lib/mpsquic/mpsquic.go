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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hostinfo"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/tools/scmp/cmn"
)

const (
	defKeyPath = "gen-certs/tls.key"
	defPemPath = "gen-certs/tls.pem"
)

const (
	maxDuration time.Duration = 1<<63 - 1
)

var _ quic.Session = (*MPQuic)(nil)

type MPQuic struct {
	quic.Session
	SCIONFlexConnection *SCIONFlexConn
	raddrs              []*snet.Addr
	paths               []spathmeta.AppPath
	active              int
	network             *snet.SCIONNetwork
	dispConn            *reliable.Conn
	raddrRTTs           []time.Duration
	raddrBW             []int // in bps
	raddrPathExp        []time.Time
}

var _ quic.Stream = (*monitoredStream)(nil)

type monitoredStream struct {
	quic.Stream
	underlayConn *MPQuic
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

func (ms monitoredStream) Write(p []byte) (n int, err error) {
	//streamID := ms.Stream.StreamID()
	start := time.Now()
	n, err = ms.Stream.Write(p)
	elapsed := time.Now().Sub(start)
	bandwidth := len(p) * 8 * 1e9 / int(elapsed)
	ms.underlayConn.raddrBW[ms.underlayConn.active] = bandwidth
	return
}

func (mpq *MPQuic) OpenStreamSync() (quic.Stream, error) {
	stream, err := mpq.Session.OpenStreamSync()
	if err != nil {
		return nil, err
	}
	return monitoredStream{stream, mpq}, nil
}

func (mpq *MPQuic) Close(err error) error {
	if mpq.Session != nil {
		return mpq.Session.Close(err)
	}
	return nil
}

func (mpq *MPQuic) CloseConn() error {
	if mpq.dispConn != nil {
		tmp := mpq.dispConn
		mpq.dispConn = nil
		time.Sleep(time.Second)
		err := tmp.Close()
		if err != nil {
			return err
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
			id := uint64(i + 1)
			info := &scmp.InfoEcho{Id: id, Seq: 0}
			pkt := cmn.NewSCMPPkt(scmp.T_G_EchoRequest, info, nil)
			b := make(common.RawBytes, mpq.paths[i].Entry.Path.Mtu)
			nhAddr := cmn.NextHopAddr()

			nextPktTS := time.Now()
			cmn.UpdatePktTS(pkt, nextPktTS)
			// Serialize packet to internal buffer
			pktLen, err := hpkt.WriteScnPkt(pkt, b)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to serialize SCION packet. err=%v\n", err)
				break
			}
			written, err := mpq.dispConn.WriteTo(b[:pktLen], nhAddr)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to write. err=%v\n", err)
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
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: Unable to read. err=%v\n", err)
				break
			}
		}
		now := time.Now()
		err = hpkt.ParseScnPkt(pkt, b[:pktLen])
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: SCION packet parse. err=%v\n", err)
			continue
		}
		// Validate scmp packet
		var scmpHdr *scmp.Hdr
		var scmpPld *scmp.Payload
		var info *scmp.InfoEcho

		scmpHdr, ok := pkt.L4.(*scmp.Hdr)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Not an SCMP header. type=%v\n", common.TypeOf(pkt.L4))
			continue
		}
		scmpPld, ok = pkt.Pld.(*scmp.Payload)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Not an SCMP payload. type=%v\n", common.TypeOf(pkt.Pld))
			continue
		}
		_ = scmpPld
		info, ok = scmpPld.Info.(*scmp.InfoEcho)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Not an Info Echo. type=%v\n", common.TypeOf(scmpPld.Info))
			continue
		}

		cmn.Stats.Recv += 1

		// Calculate RTT
		rtt := now.Sub(scmpHdr.Time()).Round(time.Microsecond)
		if info.Id-1 < uint64(len(mpq.raddrRTTs)) {
			//fmt.Println("Received SCMP packet, len:", pktLen, "ID", info.Id)
			mpq.raddrRTTs[info.Id-1] = rtt
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Wrong InfoEcho Id. id=%v\n", info.Id)
		}
	}
}

func (mpq *MPQuic) refreshPaths(resolver pathmgr.Resolver) {
	var filter *pathpol.Policy = nil
	sciondTimeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), sciondTimeout)
	defer cancel()
	syncPathMonitor, err := resolver.WatchFilter(ctx, mpq.network.IA(), mpq.raddrs[0].IA, filter)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Failed to monitor paths. src=%v, dst=%v, filter=%v\n", mpq.network.IA(), mpq.raddrs[0].IA, filter)
		syncPathMonitor = nil
	}

	if syncPathMonitor == nil {
		return
	}

	syncPathsData := syncPathMonitor.Load()
	for pathIndex, expiringPath := range mpq.paths {
		selectionKey := expiringPath.Key()
		appPath := syncPathsData.APS.GetAppPath(selectionKey)
		if appPath.Key() != selectionKey {
			_, _ = fmt.Fprintf(os.Stderr, "INFO: Failed to refresh path. Retrying later. "+
				"src=%v, dst=%v, key=%v, path=%v, filter=%v\n",
				mpq.network.IA(), mpq.raddrs[0].IA, selectionKey, expiringPath.Entry.Path.Interfaces, filter)
		} else {
			freshExpTime := time.Unix(int64(appPath.Entry.Path.ExpTime), 0)
			if freshExpTime.After(mpq.raddrPathExp[pathIndex]) {
				mpq.paths[pathIndex] = *appPath

				// Update the path on the remote address
				newPath := spath.New(appPath.Entry.Path.FwdPath)
				_ = newPath.InitOffsets()
				tmpRaddr := mpq.raddrs[pathIndex].Copy()
				tmpRaddr.Path = newPath
				tmpRaddr.NextHop, _ = appPath.Entry.HostInfo.Overlay()
				mpq.raddrs[pathIndex] = tmpRaddr
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "DEBUG: Refreshed path does not have later expiry. Retrying later. "+
					"src=%v, dst=%v, key=%v, path=%v, filter=%v, currExp=%v, freshExp=%v\n",
					mpq.network.IA(), mpq.raddrs[0].IA, selectionKey, expiringPath.Entry.Path.Interfaces, filter, mpq.raddrPathExp[pathIndex], freshExpTime)
			}
		}
	}
}

func (mpq *MPQuic) earliestPathExpiry() (ret time.Time) {
	ret = time.Now().Add(maxDuration)
	for _, exp := range mpq.raddrPathExp {
		if exp.Before(ret) {
			ret = exp
		}
	}
	return
}

func (mpq *MPQuic) managePaths() {
	lastUpdate := time.Now()
	pr := mpq.network.PathResolver()
	// Get initial expiration time of all paths
	for i, path := range mpq.paths {
		mpq.raddrPathExp[i] = time.Unix(int64(path.Entry.Path.ExpTime), 0)
	}

	// Busy wait until we have at least measurements on two paths
	for {
		var measuredPaths int
		for _, v := range mpq.raddrRTTs {
			if v != maxDuration {
				measuredPaths += 1
			}
		}
		if measuredPaths > 1 {
			break
		}
	}

	// Make a (voluntary) path change decision to increase performance at most once per 5 seconds
	// Use the time in between to refresh the path information if required
	var maxFlap time.Duration = 5 * time.Second
	for {
		if mpq.dispConn == nil {
			break
		}

		earliesExp := mpq.earliestPathExpiry()
		// Refresh the paths if one of them expires in less than 10 minutes
		if earliesExp.Before(time.Now().Add(10*time.Minute - time.Duration(rand.Intn(10))*time.Second)) {
			mpq.refreshPaths(pr)
		}

		sinceLastUpdate := time.Now().Sub(lastUpdate)
		time.Sleep(maxFlap - sinceLastUpdate) // Failing paths are handled separately / faster
		err := switchMPConn(mpq, false)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Failed to switch path. err=%v\n", err)
		}
		lastUpdate = time.Now()
	}
}

func (mpq *MPQuic) monitor() {
	cmn.Remote = *mpq.SCIONFlexConnection.raddr
	cmn.Local = *mpq.SCIONFlexConnection.laddr
	if cmn.Stats == nil {
		cmn.Stats = &cmn.ScmpStats{}
	}

	go mpq.sendSCMP()
	go mpq.rcvSCMP()

	go mpq.managePaths()
}

func DialMP(network *snet.SCIONNetwork, laddr *snet.Addr, raddr *snet.Addr, paths *[]spathmeta.AppPath,
	quicConfig *quic.Config) (*MPQuic, error) {

	return DialMPWithBindSVC(network, laddr, raddr, paths, nil, addr.SvcNone, quicConfig)
}

func DialMPWithBindSVC(network *snet.SCIONNetwork, laddr *snet.Addr, raddr *snet.Addr, paths *[]spathmeta.AppPath, baddr *snet.Addr,
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
	laddrMonitor.Host.L4 = addr.NewL4UDPInfo(laddr.Host.L4.Port() + 1)
	dispConn, _, err := reliable.Register(reliable.DefaultDispPath, laddrMonitor.IA, laddrMonitor.Host,
		overlayBindAddr, addr.SvcNone)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to register with the dispatcher addr=%s\nerr=%v", laddrMonitor, err))
	}

	var raddrs []*snet.Addr = []*snet.Addr{}
	if paths != nil {
		for _, p := range *paths {
			r := raddr.Copy()
			r.Path = spath.New(p.Entry.Path.FwdPath)
			_ = r.Path.InitOffsets()
			r.NextHop, _ = p.Entry.HostInfo.Overlay()
			raddrs = append(raddrs, r)
		}
	} else {
		paths = &[]spathmeta.AppPath{}
		con, err := parseSPath(*raddr.Path)
		if err == nil {
			pathMeta := sciond.FwdPathMeta{
				FwdPath: raddr.Path.Raw,
				Mtu: con.Mtu,
				Interfaces: con.Interfaces,
				ExpTime: uint32(con.ComputeExpTime().Unix())}
			appPath := spathmeta.AppPath{
				Entry: &sciond.PathReplyEntry{
					Path: &pathMeta,
					HostInfo:	*hostinfo.FromHostAddr(raddr.Host.L3, raddr.Host.L4.Port())}}
			*paths = append(*paths, appPath)
		}
		// No paths defined, only path information is already on the raddr
		if raddr.Path != nil {
			_ = raddr.Path.InitOffsets()
		}
		raddrs = append(raddrs, raddr)
	}

	active := 0
	if len(raddrs) < 1 {
		return nil, errors.New(fmt.Sprintf("No valid remote addresses or paths. raddr=%s\npaths=%v", raddr, paths))
	}
	flexConn := newSCIONFlexConn(sconn, laddr, raddrs[active])

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}

	raddrRTTs := []time.Duration{}
	raddrBW := []int{}
	raddrPathExp := []time.Time{}
	for _ = range raddrs {
		raddrRTTs = append(raddrRTTs, maxDuration)
		raddrBW = append(raddrBW, 0)
		raddrPathExp = append(raddrPathExp, time.Time{})
	}

	mpQuic := &MPQuic{Session: qsession, SCIONFlexConnection: flexConn, raddrs: raddrs, paths: *paths, active: active, network: network, dispConn: dispConn,
		raddrRTTs: raddrRTTs, raddrBW: raddrBW, raddrPathExp: raddrPathExp}

	mpQuic.monitor()

	return mpQuic, nil
}

func (mpq *MPQuic) displayStats() {
	for i, expTime := range mpq.raddrPathExp {
		fmt.Printf("Path %v will expire at %v.\n", i, expTime)
	}
	for i, rtt := range mpq.raddrRTTs {
		fmt.Printf("Measured RTT of %v on path %v.\n", rtt, i)
	}
	for i, bw := range mpq.raddrBW {
		fmt.Printf("Measured approximate BW of %v Mbps on path %v.\n", bw/1e6, i)
	}
}

func (mpq *MPQuic) policyLowerRTTMatch(candidate int) bool {
	if mpq.raddrRTTs[candidate] < mpq.raddrRTTs[mpq.active] {
		return true
	}
	return false
}

// This switches between different SCION paths as given by the SCION address with path structs in raddrs
// The force flag makes switching a requirement, set it when continuing to use the existing path is not an option
func switchMPConn(mpq *MPQuic, force bool) error {
	if _, set := os.LookupEnv("DEBUG"); set {
		fmt.Println("Updating to better path:")
		mpq.displayStats()
	}
	for i := range mpq.raddrs {
		if mpq.SCIONFlexConnection.raddr != mpq.raddrs[i] && mpq.policyLowerRTTMatch(i) {
			// fmt.Printf("Previous path: %v\n", mpq.SCIONFlexConnection.raddr.Path)
			// fmt.Printf("New path: %v\n", mpq.raddrs[i].Path)
			mpq.SCIONFlexConnection.SetRemoteAddr(mpq.raddrs[i])
			mpq.active = i
			return nil
		}
	}
	if !force {
		return nil
	}

	return common.NewBasicError("mpsquic: No fallback connection available.", nil)
}
