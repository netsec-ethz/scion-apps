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

// Package mpsquic is a prototype implementation for a QUIC/SCION "socket" with
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

func Dial(remote string, tlsConf *tls.Config, quicConf *quic.Config) (*MPQuic, error) {

	raddr, err := appnet.ResolveUDPAddr(appnet.UnmangleSCIONAddr(remote))
	if err != nil {
		return nil, err
	}

	paths, err := appnet.QueryPaths(raddr.IA)
	if err != nil {
		return nil, err
	}

	paths = demoFilterPaths(paths)

	s, err := DialAddr(raddr, remote, paths, tlsConf, quicConf)
	return s, err
}

// DialAddr creates a monitored multiple paths connection using QUIC.
// It returns a MPQuic struct if a opening a QUIC session over the initial SCION path succeeded.
func DialAddr(raddr *snet.UDPAddr, host string, paths []snet.Path,
	tlsConf *tls.Config, quicConf *quic.Config) (*MPQuic, error) {

	ctx := context.Background()
	// Buffered channel, we can buffer up to 1 revocation per 20ms for 1s per path.
	revocationQ := make(chan *path_mgmt.SignedRevInfo, 50*len(paths))
	revHandler := &revocationHandler{revocationQ}

	ts := time.Now()
	qsess, active, flexConn, err := raceDial(ctx, revHandler, raddr, host, paths, tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	fmt.Println("Dialed", "num paths:", len(paths), "active:", active, "dt:", time.Since(ts))
	demoDisplayPaths(paths, active)

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
func raceDial(ctx context.Context, revHandler snet.RevocationHandler,
	raddr *snet.UDPAddr, host string, paths []snet.Path,
	tlsConf *tls.Config, quicConf *quic.Config) (quic.Session, int, *flexConn, error) {

	conns := make([]*flexConn, len(paths))
	for i, path := range paths {
		conn, err := listenWithRevHandler(ctx, revHandler)
		if err != nil {
			return nil, 0, nil, err
		}
		conns[i] = newFlexConn(conn, raddr, path)
	}

	logger.Info("Racing handshake over paths", "n", len(paths))

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

	/*
		for i := range conns {
			if i != firstID || firstSession != nil {
				conns[i].Close()
			}
		}
	*/

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
		rttStr := "-"
		if pathInfo.rtt > 0 && pathInfo.rtt < maxDuration {
			rttStr = fmt.Sprintf("%3dms", pathInfo.rtt.Milliseconds())
		}
		f := ""
		if i == mpq.active {
			f = " [active]"
		}
		fmt.Printf("Path %9s%2d: RTT %s\n", f, i, rttStr)
		/*logger.Debug(fmt.Sprintf("Path %v", i),
		"expiry", time.Until(pathInfo.path.Expiry()).Round(time.Second),
		"revoked", pathInfo.revoked,
		"RTT", rttStr,
		"approxBW [Mbps]", pathInfo.bw/1e6)
		*/
	}
}

// updateActivePath updates the active path
func (mpq *MPQuic) updateActivePath(newPathIndex int) {
	mpq.active = newPathIndex
	mpq.flexConn.SetPath(mpq.paths[newPathIndex].path)
}

func demoDisplayPaths(paths []snet.Path, active int) {

	for i, p := range paths {
		desc := demoPathDescription(p)
		f := ""
		if i == active {
			f = " [active]"
		}

		fmt.Printf("Path %2d%9s: %s\n", i, f, desc)
	}
}

func demoPathDescription(path snet.Path) string {
	asLat := mustParseIA("17-ffaa:0:1110")
	asLoss := mustParseIA("17-ffaa:0:1111")
	asBW := mustParseIA("17-ffaa:0:1112")
	snippets := map[pathInterface]string{
		{asLat, 1}:  "low latency",
		{asLat, 2}:  "mid latency",
		{asLat, 3}:  "high latency",
		{asLoss, 4}: "no loss",
		{asLoss, 5}: "low loss",
		{asLoss, 6}: "intermittent",
		{asBW, 4}:   "low bw",
		{asBW, 5}:   "mid bw",
		{asBW, 6}:   "high bw",
	}

	desc := ""
	ifaces := path.Interfaces()
	for i, iface := range ifaces {
		if snippet, ok := snippets[pathInterface{iface.IA(), uint64(iface.ID())}]; ok {
			if desc != "" {
				desc += ", "
			}
			desc += snippet
		} else if name, ok := demoASNames[iface.IA().String()]; ok && (i == 0 || i%2 == 1) {
			if desc != "" {
				desc += " > "
			}
			desc += name
		}
	}
	return desc
}

var demoASNames = map[string]string{
	"16-ffaa:0:1001": "AWS Frankfurt",
	"16-ffaa:0:1002": "AWS Ireland",
	"16-ffaa:0:1003": "AWS US N. Virginia",
	"16-ffaa:0:1004": "AWS US Ohio",
	"16-ffaa:0:1005": "AWS US Oregon",
	"16-ffaa:0:1006": "AWS Japan",
	"16-ffaa:0:1007": "AWS Singapore",
	"16-ffaa:0:1008": "AWS Oregon non-core",
	"16-ffaa:0:1009": "AWS Frankfurt non-core",
	"17-ffaa:0:1101": "SCMN",
	"17-ffaa:0:1102": "ETHZ",
	"17-ffaa:0:1103": "SWITCHEngine Zurich",
	"17-ffaa:0:1107": "ETHZ-AP",
	"17-ffaa:0:1108": "SWITCH",
	"18-ffaa:0:1201": "CMU",
	"18-ffaa:0:1203": "Columbia",
	"18-ffaa:0:1204": "ISG Toronto",
	"18-ffaa:0:1206": "CMU AP",
	"19-ffaa:0:1301": "Magdeburg core",
	"19-ffaa:0:1302": "GEANT",
	"19-ffaa:0:1303": "Magdeburg AP",
	"19-ffaa:0:1304": "FR@Linode",
	"19-ffaa:0:1305": "SIDN",
	"19-ffaa:0:1306": "Deutsche Telekom",
	"19-ffaa:0:1307": "TW Wien",
	"19-ffaa:0:1309": "Valencia",
	"19-ffaa:0:130a": "IMDEA Madrid",
	"19-ffaa:0:130b": "DFN",
	"19-ffaa:0:130c": "Grid5000",
	"19-ffaa:0:130d": "Aalto University",
	"19-ffaa:0:130e": "Aalto University II",
	"19-ffaa:0:130f": "Centria UAS Finland",
	"20-ffaa:0:1401": "KISTI Daejeon",
	"20-ffaa:0:1402": "KISTI Seoul",
	"20-ffaa:0:1403": "KAIST",
	"20-ffaa:0:1404": "KU",
	"21-ffaa:0:1501": "KDDI",
	"22-ffaa:0:1601": "NTU",
	"23-ffaa:0:1701": "NUS",
	"25-ffaa:0:1901": "THU",
	"25-ffaa:0:1902": "CUHK",
	"26-ffaa:0:2001": "KREONET2 Worldwide",
}

func demoFilterPaths(paths []snet.Path) []snet.Path {

	asLat := mustParseIA("17-ffaa:0:1110")
	asLoss := mustParseIA("17-ffaa:0:1111")
	asBW := mustParseIA("17-ffaa:0:1112")

	exclusionRules := [][]pathInterface{
		{{asBW, 4}},               // avoid 200kbps link
		{{asBW, 5}},               //    "  2Mbps
		{{asLat, 1}, {asLoss, 4}}, // allow use of low latency links only in combination with intermittent 100% lossy link (at remaining interface 6)
		{{asLat, 1}, {asLoss, 5}},
		{{asLat, 2}, {asLoss, 4}},
		{{asLat, 2}, {asLoss, 5}},
	}

	filtered := make([]snet.Path, 0, len(paths))
	for _, p := range paths {
		// match no exclusion rules
		excluded := false
		for _, rule := range exclusionRules {
			if containsAllInterfaces(p, rule) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// containsAllInterfaces returns true if path contains all interfaces in interface list
func containsAllInterfaces(path snet.Path, ifaceList []pathInterface) bool {
	ifaceSet := pathInterfaceSet(path)
	for _, iface := range ifaceList {
		if _, exists := ifaceSet[iface]; !exists {
			return false
		}
	}
	return true
}

func pathInterfaceSet(path snet.Path) map[pathInterface]struct{} {
	set := make(map[pathInterface]struct{})
	for _, iface := range path.Interfaces() {
		set[pathInterface{iface.IA(), uint64(iface.ID())}] = struct{}{}
	}
	return set
}

type pathInterface struct {
	ia addr.IA
	id uint64
}

func mustParseIA(iaStr string) addr.IA {
	ia, err := addr.IAFromString(iaStr)
	if err != nil {
		panic(err)
	}
	return ia
}
