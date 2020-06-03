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

// package mpsquic is a prototype implementation for a QUIC/SCION "socket" with
// automatic, performance aware path choice.
package mpsquic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
)

const (
	maxDuration time.Duration = 1<<63 - 1
)

var _ quic.Session = (*MPQuic)(nil)

// XXX(matzf): redundant fields? raddr contains path, path contains fingerprint/expiry.
type pathInfo struct {
	raddr       *snet.UDPAddr
	path        snet.Path
	fingerprint snet.PathFingerprint // caches path.Fingerprint()
	expiry      time.Time
	rtt         time.Duration
	bw          int // in bps
}

// TODO(matzf): rename to Session?
type MPQuic struct {
	quic.Session
	flexConn     *flexConn
	dispConn     net.PacketConn
	paths        []*pathInfo
	active       *pathInfo
	pathResolver pathmgr.Resolver
	revocationQ  chan *path_mgmt.SignedRevInfo
}

// OpenStreamSync opens a QUIC stream over the QUIC session.
// It returns a QUIC stream ready to be written/read.
func (mpq *MPQuic) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	stream, err := mpq.Session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return monitoredStream{stream, mpq}, nil
}

// Close closes the QUIC session.
func (mpq *MPQuic) CloseWithError(code quic.ErrorCode, desc string) error {
	// TODO(matzf) return all errors (multierr)
	if mpq.Session != nil {
		if err := mpq.Session.CloseWithError(code, desc); err != nil {
			logger.Warn("Error closing QUIC session", "err", err)
		}
	}
	mpq.dispConn.Close()
	return mpq.flexConn.Close()
}

// createSCMPMonitorConn opens a new connection to send/receive SCMPs on.
// NOTE(matzf): this should not be necessary, as the regular snet.Conn uses the
// same underlying type. We just cant get it out.
func createSCMPMonitorConn(ctx context.Context) (net.PacketConn, error) {
	// New connection
	// Ignore assigned port
	disp := appnet.DefNetwork().Dispatcher
	localIA := appnet.DefNetwork().IA
	localIP, err := appnet.DefaultLocalIP()
	if err != nil {
		return nil, err
	}
	dispConn, _, err := disp.Register(ctx, localIA, &net.UDPAddr{IP: localIP, Port: 0}, addr.SvcNone)
	return dispConn, err
}

// Dial creates a monitored multiple paths connection using QUIC.
// It returns a MPQuic struct if a opening a QUIC session over the initial SCION path succeeded.
func Dial(raddr *snet.UDPAddr, host string, paths []snet.Path,
	tlsConf *tls.Config, quicConf *quic.Config) (*MPQuic, error) {

	ctx := context.Background()

	// Buffered channel, we can buffer up to 1 revocation per 20ms for 1s.
	revocationQ := make(chan *path_mgmt.SignedRevInfo, 50)

	conn, err := listenWithRevHandler(ctx, &revocationHandler{revocationQ})
	if err != nil {
		return nil, err
	}

	dispConn, err := createSCMPMonitorConn(ctx)
	if err != nil {
		return nil, err
	}

	// XXX(matzf): make this public on DefNetwork
	sdConn := appnet.DefNetwork().PathQuerier.(sciond.Querier).Connector
	pathResolver := pathmgr.New(sdConn, pathmgr.Timers{}, 0)

	pathInfos := makePathInfos(paths, raddr)

	active := pathInfos[0]
	flexConn := newFlexConn(conn, active.raddr)
	qsession, err := quic.Dial(flexConn, flexConn.raddr, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	mpQuic := &MPQuic{
		Session:      qsession,
		flexConn:     flexConn,
		dispConn:     dispConn,
		paths:        pathInfos,
		active:       active,
		pathResolver: pathResolver,
		revocationQ:  revocationQ,
	}
	logger.Info("Active Path", "key", active.fingerprint, "Hops", active.path.Interfaces())
	mpQuic.monitor()

	return mpQuic, nil
}

// listenWithRevHandler is analogous to appnet.Listen(nil), but also sets a
// custom revocation handler on the connection.
func listenWithRevHandler(ctx context.Context, revHandler snet.RevocationHandler) (*snet.Conn, error) {

	// this is ugly as; the revHandler could be configured per connection but
	// it's not accessible so we have to make this weird detour of creating a new
	// Network object.
	defNetwork := appnet.DefNetwork()
	network := snet.NewNetworkWithPR(
		defNetwork.IA,
		defNetwork.Dispatcher,
		nil, // unused, this will go away
		revHandler,
	)
	// Analogous to appnet.Listen(nil), but need to hand roll because we are not
	// using the default network
	localIP, err := appnet.DefaultLocalIP()
	if err != nil {
		return nil, err
	}
	return network.Listen(ctx, "udp", &net.UDPAddr{IP: localIP, Port: 0}, addr.SvcNone)
}

// makePathInfos initializes pathInfo structs for the paths
func makePathInfos(paths []snet.Path, raddr *snet.UDPAddr) []*pathInfo {

	pathInfos := make([]*pathInfo, 0, len(paths))
	for i, p := range paths {
		logger.Info("Path", "index", i, "interfaces", p.Interfaces())
		r := raddr.Copy()
		r.Path = p.Path()
		r.NextHop = p.OverlayNextHop()

		pi := &pathInfo{
			raddr:       r,
			path:        p,
			fingerprint: p.Fingerprint(),
			expiry:      p.Expiry(),
			rtt:         maxDuration,
			bw:          0,
		}
		pathInfos = append(pathInfos, pi)
	}
	return pathInfos
}

// displayStats logs the collected metrics for all monitored paths.
func (mpq *MPQuic) displayStats() {
	for i, pathInfo := range mpq.paths {
		logger.Debug(fmt.Sprintf("Path %v will expire at %v.\n", i, pathInfo.expiry))
		logger.Debug(fmt.Sprintf("Measured RTT of %v on path %v.\n", pathInfo.rtt, i))
		logger.Debug(fmt.Sprintf("Measured approximate BW of %v Mbps on path %v.\n", pathInfo.bw/1e6, i))
	}
}

// policyLowerRTTMatch returns true if the path with candidate index has a lower RTT than the active path.
func (mpq *MPQuic) policyLowerRTTMatch(candidate int) bool {
	return mpq.paths[candidate].rtt < mpq.active.rtt
}

// updateActivePath updates the active path in a thread safe manner.
func (mpq *MPQuic) updateActivePath(newPathIndex int) {
	// Lock the connection raddr, and update both the active path and the raddr of the FlexConn.
	mpq.flexConn.addrMtx.Lock()
	defer mpq.flexConn.addrMtx.Unlock()
	mpq.active = mpq.paths[newPathIndex]
	mpq.flexConn.setRemoteAddr(mpq.active.raddr)
}

// switchMPConn switches between different SCION paths as given by the SCION address with path structs in paths.
// The force flag makes switching a requirement, set it when continuing to use the existing path is not an option.
func (mpq *MPQuic) switchMPConn(force bool, filter bool) error {
	mpq.displayStats()
	if force {
		// Always refresh available paths, as failing to find a fresh path leads to a hard failure
		mpq.refreshPaths(mpq.pathResolver)
	}
	for i := range mpq.paths {
		// Do not switch to identical path or to expired path
		if mpq.flexConn.raddr != mpq.paths[i].raddr && mpq.paths[i].expiry.After(time.Now()) {
			logger.Trace("Previous path", "path", mpq.flexConn.raddr.Path)
			logger.Trace("New path", "path", mpq.paths[i].raddr.Path)
			if !filter {
				mpq.updateActivePath(i)
				logger.Debug("Updating to path", "index", i, "path", mpq.paths[i].path.Interfaces())
				return nil
			}
			if mpq.policyLowerRTTMatch(i) {
				mpq.updateActivePath(i)
				logger.Debug("Updating to better path", "index", i, "path", mpq.paths[i].path.Interfaces())
				return nil
			}
		}
	}
	if !force {
		return nil
	}
	logger.Debug("No path available now", "now", time.Now())
	mpq.displayStats()

	return errors.New("no fallback path available")
}
