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

// Package nesquic is a prototype implementation for a QUIC/SCION "socket" with
// automatic, performance aware path choice.
//
// The most important design decision/constraint for this package is to make
// use of an _unmodified_ QUIC implementation (lucas-clemente/quic-go).
// The rational is that it's not realistically feasible to maintain a modified
// fork of a QUIC implementation with the resources available.
// As consequence of this choice, we are limited to using one SCION path at a
// time (per session), i.e. it's not feasible to use multiple paths _concurrently_.
//
// Another design choice is to keep the multi-path QUIC clients compatible with
// unmodified QUIC servers -- whether this is necessary is debatable as there
// are not many SCION/QUIC servers, modified or unmodified.
//
//
// The path selection behaviour is implemented only for clients (Dial).
// The server part (Listen) uses a simpler mechanism which sends reply packets over
// the path on which the last packet from the client was received.
//
// The client's multi-path behaviour is implemented in a sandwich around the quic-go API;
// - on top:      interface layer that intercepts the relevant API calls
//                so we can insert our customized types
// - below:       the "socket" used by QUIC to send UDP packets, customized so the SCION path
//                can be actively overridden from "outside"
// - on the side: a monitor that actively probes available paths and chooses a "best" active path
//                using a "pluggable" policy.
//                The monitor reevaluates the path choice at regular time intervals, or when
//                the probing observes drastic changes (currently: revocation or timeout for
//                active path).
package nesquic

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

// maxTime is the maximum usable time value (https://stackoverflow.com/a/32620397)
var maxTime = time.Unix(1<<63-62135596801, 999999999)

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

	ts := time.Now()
	qsess, active, flexConn, err := raceDial(ctx, conn, raddr, host, paths, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	logger.Info("Dialed", "active", active, "dt", time.Since(ts))

	// TODO(matzf) defer creating this
	pinger, err := NewPinger(ctx, revHandler)
	if err != nil {
		return nil, err
	}

	policy := &lowestRTT{}
	pathInfos := makePathInfos(paths)

	mpQuic := &MPQuic{
		Session:     qsess,
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

	go mpQuic.monitor(time.Now().Add(lowestRTTReevaluateInterval))

	return mpQuic, nil
}

// raceDial dials a quic session on every path and returns the session for
// which the succeeded returned first.
func raceDial(ctx context.Context, conn *snet.Conn,
	raddr *snet.UDPAddr, host string, paths []snet.Path,
	tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, int, *flexConn, error) {

	conns := make([]*flexConn, len(paths))
	for i, path := range paths {
		conns[i] = newFlexConn(conn, raddr, path)
	}

	type indexedSessionOrError struct {
		id      int
		session quic.Session
		err     error
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan indexedSessionOrError)
	for i := range paths {
		go func(id int) {
			sess, err := quic.DialContext(ctx, conns[id], raddr, host, tlsConf, quicConf)
			results <- indexedSessionOrError{id, sess, err}
		}(i)
	}

	var firstID int
	var firstSession quic.Session
	var errs []error
	for range paths {
		result := <-results
		if result.err == nil {
			if firstSession == nil {
				firstSession = result.session
				firstID = result.id
				cancel() // abort all other Dials
			} else {
				// Dial succeeded without cancelling and not first? Unlucky, just close this session.
				// XXX(matzf) wrong layer; error code is supposed to be application layer
				_ = result.session.CloseWithError(quic.ErrorCode(0), "")
			}
		} else {
			errs = append(errs, result.err)
		}
	}

	if firstSession != nil {
		return firstSession, firstID, conns[firstID], nil
	} else {
		return nil, 0, nil, errs[0] // return first error (multierr?)
	}
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
	for _, p := range paths {

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
