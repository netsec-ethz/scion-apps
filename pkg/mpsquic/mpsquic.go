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
	"fmt"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/snet"
)

const (
	maxDuration time.Duration = 1<<63 - 1
)

var _ quic.Session = (*MPQuic)(nil)

type pathInfo struct {
	path        snet.Path
	fingerprint snet.PathFingerprint // caches path.Fingerprint()
	revoked     bool
	rtt         time.Duration
	bw          int // in bps
}

// TODO(matzf): rename to Session?
type MPQuic struct {
	quic.Session
	raddr       *snet.UDPAddr
	flexConn    *flexConn
	pinger      *Pinger
	policy      Policy
	paths       []*pathInfo
	active      int
	revocationQ chan *path_mgmt.SignedRevInfo
	probeUpdate chan []time.Duration
	stop        chan struct{}
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
	close(mpq.stop)
	_ = mpq.pinger.Close()
	return mpq.flexConn.Close()
}

// Dial creates a monitored multiple paths connection using QUIC.
// It returns a MPQuic struct if a opening a QUIC session over the initial SCION path succeeded.
func Dial(raddr *snet.UDPAddr, host string, paths []snet.Path,
	tlsConf *tls.Config, quicConf *quic.Config) (*MPQuic, error) {

	ctx := context.Background()
	// Buffered channel, we can buffer up to 1 revocation per 20ms for 1s.
	revocationQ := make(chan *path_mgmt.SignedRevInfo, 50)
	revHandler := &revocationHandler{revocationQ}
	conn, err := listenWithRevHandler(ctx, revHandler)
	if err != nil {
		return nil, err
	}
	pinger, err := newPinger(ctx, revHandler)
	if err != nil {
		return nil, err
	}

	policy := &lowestRTT{}
	pathInfos := makePathInfos(paths)
	active, nextSelectTime := policy.Select(pathInfos)
	logger.Debug("Active Path",
		"index", active,
		"key", pathInfos[active].fingerprint,
		"hops", pathInfos[active].path.Interfaces())
	flexConn := newFlexConn(conn, raddr, pathInfos[active].path)
	qsession, err := quic.Dial(flexConn, raddr, host, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}

	mpQuic := &MPQuic{
		Session:     qsession,
		raddr:       raddr,
		flexConn:    flexConn,
		pinger:      pinger,
		policy:      policy,
		paths:       pathInfos,
		active:      active,
		revocationQ: revocationQ,
		probeUpdate: make(chan []time.Duration),
		stop:        make(chan struct{}),
	}

	go mpQuic.monitor(nextSelectTime)

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
func makePathInfos(paths []snet.Path) []*pathInfo {

	pathInfos := make([]*pathInfo, 0, len(paths))
	for i, p := range paths {
		logger.Info("Path", "index", i, "interfaces", p.Interfaces())

		pi := &pathInfo{
			path:        p,
			fingerprint: p.Fingerprint(),
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
		logger.Debug(fmt.Sprintf("Path %v stats", i),
			"expiry", time.Until(pathInfo.path.Expiry()).Round(time.Second),
			"revoked", pathInfo.revoked,
			"RTT", pathInfo.rtt,
			"approxBW [Mbps]", pathInfo.bw/1e6)
	}
}

// updateActivePath updates the active path
func (mpq *MPQuic) updateActivePath(newPathIndex int) {
	mpq.active = newPathIndex
	mpq.flexConn.SetPath(mpq.paths[newPathIndex].path)
}
