package nesquic

import (
	"math/rand"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/snet"
)

const echoRequestInterval = 500 * time.Millisecond
const pathExpiryRefreshLeadTime = 10 * time.Minute

// pathRefreshMinInterval is the minimum delay between two path refreshs
const pathRefreshMinInterval = 10 * time.Second

var _ quic.Stream = (*monitoredStream)(nil)

type monitoredStream struct {
	quic.Stream
	session *MPQuic
}

// Write writes to a quic Stream while monitoring its approximate bandwidth
func (ms monitoredStream) Write(p []byte) (n int, err error) {
	//streamID := ms.Stream.StreamID()
	activeAtWriteStart := ms.session.active
	start := time.Now()

	n, err = ms.Stream.Write(p)

	elapsed := time.Since(start)
	bandwidth := len(p) * 8 * 1e9 / int(elapsed)
	// Check if the path remained the same
	if ms.session.active == activeAtWriteStart {
		ms.session.paths[activeAtWriteStart].bw = bandwidth
	}
	return
}

// monitor evaluates the path choice policy, keeps path information up to date and probes
// the paths' RTTs.
// It manages path expiration and path change decisions.
func (mpq *MPQuic) monitor(firstSelect time.Time) {
	go mpq.sendPings()
	mpq.managePaths(firstSelect)
}

// monitor evaluates the path choice policy at the specified intervals, processes path revocations
// and refreshes path information.
func (mpq *MPQuic) managePaths(firstSelect time.Time) {

	selectTimer := time.NewTimer(time.Until(firstSelect))
	defer selectTimer.Stop()
	refreshTimer := time.NewTimer(time.Until(mpq.nextPathRefresh()))
	defer refreshTimer.Stop()
	for {
		select {
		case <-mpq.stop:
			break
		case rev := <-mpq.revocationQ:
			activePathRevoked := mpq.handleRevocation(rev)
			if activePathRevoked {
				logger.Trace("Processed revocation for active path")
				mpq.selectPath(selectTimer)
			}
		case rtts := <-mpq.probeUpdate:
			activePathTimeout := (rtts[mpq.active] == maxDuration &&
				mpq.paths[mpq.active].rtt < maxDuration)
			for i := range mpq.paths {
				mpq.paths[i].rtt = rtts[i]
			}
			if activePathTimeout {
				mpq.selectPath(selectTimer)
			}
		case <-selectTimer.C:
			mpq.selectPath(selectTimer)
		case <-refreshTimer.C:
			mpq.refreshPaths()
			refreshTimer.Reset(time.Until(mpq.nextPathRefresh()))
		}
	}
}

// selectPath evaluates the path choice policy and resets the timer
func (mpq *MPQuic) selectPath(selectTimer *time.Timer) {

	i, nextTime := mpq.policy.Select(mpq.active, mpq.paths)
	if i != mpq.active {
		mpq.displayStats()
		mpq.updateActivePath(i)
		logger.Debug("Changed active path", "active", i)
	}

	expiry := mpq.paths[i].path.Expiry()
	if expiry.Before(nextTime) {
		nextTime = expiry
	}
	resetTimer(selectTimer, nextTime)
}

// sendPings sends SCMP echo messages on all paths at regular intervals
func (mpq *MPQuic) sendPings() {

	var appID uint64 = rand.Uint64()

	t := time.NewTicker(echoRequestInterval)
	defer t.Stop()

	var seq uint16
	for {
		select {
		case <-mpq.stop:
			break
		case <-t.C:
			raddrs := make([]*snet.UDPAddr, len(mpq.paths))
			for i := range mpq.paths {
				raddrs[i] = mpq.raddr.Copy()
				appnet.SetPath(raddrs[i], mpq.paths[i].path)
			}
			rtts, err := mpq.pinger.PingAll(raddrs, appID, seq, echoRequestInterval)
			if err != nil {
				logger.Error("Error probing paths", "err", err, "errType", common.TypeOf(err))
			} else {
				logger.Trace("Paths pinged", "rtts", rtts, "seq", seq)
				mpq.probeUpdate <- rtts
			}

			seq++
		}
	}
}

// refreshPaths requests sciond for updated paths
func (mpq *MPQuic) refreshPaths() {
	freshPaths, err := appnet.QueryPaths(mpq.flexConn.raddr.IA)
	if err != nil {
		logger.Error("Failed to query paths", "err", err)
		return
	}
	freshPathSet := make(map[snet.PathFingerprint]snet.Path)
	for _, p := range freshPaths {
		freshPathSet[p.Fingerprint()] = p
	}

	for _, pathInfo := range mpq.paths {
		// Update paths for which fresh information was returned.
		// Expired or revoked paths are missing from the fresh paths.
		if fresh, ok := freshPathSet[pathInfo.fingerprint]; ok {
			if fresh.Expiry().After(pathInfo.path.Expiry()) {
				// Update the path on the remote address
				pathInfo.path = fresh
				pathInfo.revoked = false
			} else {
				logger.Debug("Refreshed path does not have later expiry. Retrying later.")
			}
		}
	}
}

// earliestPathExpiry computes the earliest expiration time of any path registered in mpq.
func (mpq *MPQuic) earliestPathExpiry() time.Time {
	ret := maxTime
	for _, pathInfo := range mpq.paths {
		if pathInfo.path.Expiry().Before(ret) {
			ret = pathInfo.path.Expiry()
		}
	}
	return ret
}

// nextPathRefresh the time for the next scheduled path refresh, based on the
// expiration of the paths
func (mpq *MPQuic) nextPathRefresh() time.Time {
	expiry := mpq.earliestPathExpiry()
	randOffset := time.Duration(rand.Intn(10)) * time.Second
	nextRefresh := expiry.Add(-pathExpiryRefreshLeadTime + randOffset)

	// if are still paths that expire very soon (or have already expired), we
	// still wait a little bit until the next refresh. Otherwise, failing refresh
	// of an expired path would make us refresh continuously.
	earliestAllowed := time.Now().Add(pathRefreshMinInterval)
	if nextRefresh.Before(earliestAllowed) {
		return earliestAllowed
	}
	return nextRefresh
}

// resetTimer resets the timer, as described in godoc for time.Timer.Reset.
//
// This cannot be done concurrent to other receives from the Timer's channel or
// other calls to the Timer's Stop method.
func resetTimer(t *time.Timer, when time.Time) {
	if !t.Stop() {
		// Drain the event channel if not empty
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(time.Until(when))
}
