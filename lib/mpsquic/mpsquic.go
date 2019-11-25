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
	"github.com/scionproto/scion/go/lib/hostinfo"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

const (
	defKeyPath = "gen-certs/tls.key"
	defPemPath = "gen-certs/tls.pem"
)

const (
	maxDuration time.Duration = 1<<63 - 1
)

var _ quic.Session = (*MPQuic)(nil)

type pathInfo struct {
	raddr      *snet.Addr
	path       spathmeta.AppPath
	expiration time.Time
	rtt        time.Duration
	bw         int // in bps
}

type MPQuic struct {
	quic.Session
	SCIONFlexConnection *SCIONFlexConn
	network             *snet.SCIONNetwork
	dispConn            *reliable.Conn
	paths               []pathInfo
	active              *pathInfo
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
		// Infer path meta information from path on raddr, since no paths were provided
		con, err := parseSPath(*raddr.Path)
		if err == nil {
			pathMeta := sciond.FwdPathMeta{
				FwdPath:    raddr.Path.Raw,
				Mtu:        con.Mtu,
				Interfaces: con.Interfaces,
				ExpTime:    uint32(con.ComputeExpTime().Unix())}
			appPath := spathmeta.AppPath{
				Entry: &sciond.PathReplyEntry{
					Path:     &pathMeta,
					HostInfo: *hostinfo.FromHostAddr(raddr.Host.L3, raddr.Host.L4.Port())}}
			*paths = append(*paths, appPath)
		}
		if raddr.Path != nil {
			_ = raddr.Path.InitOffsets()
		}
		raddrs = append(raddrs, raddr)
	}

	if len(raddrs) < 1 {
		return nil, errors.New(fmt.Sprintf("No valid remote addresses or paths. raddr=%s\npaths=%v", raddr, paths))
	}

	pathInfos := []pathInfo{}
	for i, raddr := range raddrs {
		pi := pathInfo{
			raddr:      raddr,
			path:       (*paths)[i],
			expiration: time.Time{},
			rtt:        maxDuration,
			bw:         0,
		}
		pathInfos = append(pathInfos, pi)
	}

	active := &pathInfos[0]
	flexConn := newSCIONFlexConn(sconn, laddr, active.raddr)

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}

	mpQuic := &MPQuic{Session: qsession, SCIONFlexConnection: flexConn, network: network, dispConn: dispConn, paths: pathInfos, active: active}

	mpQuic.monitor()

	return mpQuic, nil
}

func (mpq *MPQuic) displayStats() {
	for i, pathInfo := range mpq.paths {
		fmt.Printf("Path %v will expire at %v.\n", i, pathInfo.expiration)
		fmt.Printf("Measured RTT of %v on path %v.\n", pathInfo.rtt, i)
		fmt.Printf("Measured approximate BW of %v Mbps on path %v.\n", pathInfo.bw/1e6, i)
	}
}

func (mpq *MPQuic) policyLowerRTTMatch(candidate int) bool {
	if mpq.paths[candidate].rtt < mpq.active.rtt {
		return true
	}
	return false
}

func (mpq *MPQuic) updateActivePath(newPathIndex int) {
	// Lock the connection raddr, and update both the active path and the raddr of the FlexConn
	mpq.SCIONFlexConnection.addrMtx.Lock()
	defer mpq.SCIONFlexConnection.addrMtx.Unlock()
	mpq.active = &mpq.paths[newPathIndex]
	mpq.SCIONFlexConnection.setRemoteAddr(mpq.active.raddr)
}

// This switches between different SCION paths as given by the SCION address with path structs in paths
// The force flag makes switching a requirement, set it when continuing to use the existing path is not an option
func switchMPConn(mpq *MPQuic, force bool) error {
	if _, set := os.LookupEnv("DEBUG"); set { // TODO: Remove this when cleaning up logging
		fmt.Println("Updating to better path:")
		mpq.displayStats()
	}
	for i := range mpq.paths {
		if mpq.SCIONFlexConnection.raddr != mpq.paths[i].raddr && mpq.policyLowerRTTMatch(i) {
			// fmt.Printf("Previous path: %v\n", mpq.SCIONFlexConnection.raddr.Path)
			// fmt.Printf("New path: %v\n", mpq.paths[i].raddr.Path)
			mpq.updateActivePath(i)
			return nil
		}
	}
	if !force {
		return nil
	}

	return common.NewBasicError("mpsquic: No fallback connection available.", nil)
}
