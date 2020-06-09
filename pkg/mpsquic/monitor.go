package mpsquic

import (
	"context"
	"math/rand"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"

	"github.com/scionproto/scion/go/lib/pathmgr"
)

const echoRequestInterval = 500 * time.Millisecond
const pathExpiryRefreshLeadTime = 10 * time.Minute

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
	var appID uint64 = rand.Uint64()
	go mpq.sendPings(appID)
	go mpq.rcvPings(appID)
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
			logger.Trace("Processing revocation", "action", "Retrieved queued revocation")
			activePathRevoked := mpq.handleRevocation(rev)
			if activePathRevoked {
				mpq.selectPath(selectTimer)
			}
		case <-selectTimer.C:
			mpq.selectPath(selectTimer)
		case <-refreshTimer.C:
			mpq.refreshPaths(mpq.pathResolver)
			refreshTimer.Reset(time.Until(mpq.nextPathRefresh()))
		}
	}
}

// selectPath evaluates the path choice policy and resets the timer
func (mpq *MPQuic) selectPath(selectTimer *time.Timer) {

	mpq.displayStats()

	i, nextTime := mpq.policy.Select(mpq.paths)
	if i != mpq.active {
		mpq.updateActivePath(i)
		logger.Debug("Changed active path",
			"index", i,
			"key", mpq.paths[i].fingerprint,
			"hops", mpq.paths[i].path.Interfaces())
	}

	if mpq.paths[i].expiry.Before(nextTime) {
		nextTime = mpq.paths[i].expiry
	}
	resetTimer(selectTimer, nextTime)
}

// sendPings sends SCMP echo messages on all paths at regular intervals
func (mpq *MPQuic) sendPings(appID uint64) {

	t := time.NewTicker(echoRequestInterval)
	defer t.Stop()

	var seq uint16
	for {
		select {
		case <-mpq.stop:
			break
		case <-t.C:
			for i := range mpq.paths {
				scmpID := appID + uint64(i)
				err := mpq.pinger.Ping(mpq.paths[i].raddr,
					scmpID,
					seq)
				if err != nil {
					logger.Error("Error sending SCMP echo", "err", err)
				} else {
					logger.Trace("Sent SCMP echo", "id", scmpID, "Seq", seq)
				}
			}
			seq++
		}
	}
}

// rcvPings reads SCMP echo reply messages
func (mpq *MPQuic) rcvPings(appID uint64) {
	for {
		select {
		case <-mpq.stop:
			break
		default:
			mpq.rcvPing(appID)
		}
	}
}

// rcvPing receives and processes one SCMP echo reply message
func (mpq *MPQuic) rcvPing(appID uint64) {
	reply, err := mpq.pinger.ReadReply()
	if err != nil {
		logger.Error("Unable to read echo reply", "err", err)
		return
	}

	scmpID := reply.ID
	if scmpID >= appID && scmpID < appID+uint64(len(mpq.paths)) {
		logger.Trace("Received SCMP echo reply", "id", scmpID, "seq", reply.Seq, "RTT", reply.RTT)
		pathID := scmpID - appID
		mpq.paths[pathID].rtt = reply.RTT
		mpq.paths[pathID].revoked = false // if it works, it works
	} else {
		logger.Error("Unexpected SCMP echo reply", "id", scmpID, "appID", appID)
	}
}

// refreshPaths requests sciond for updated paths
func (mpq *MPQuic) refreshPaths(resolver pathmgr.Resolver) {
	sciondTimeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), sciondTimeout)
	defer cancel()
	localIA := appnet.DefNetwork().IA
	syncPathMonitor, err := resolver.Watch(ctx, localIA, mpq.paths[0].raddr.IA)
	if err != nil {
		logger.Error("Failed to monitor paths.", "src", localIA, "dst", mpq.paths[0].raddr.IA)
		return
	}

	syncPathsData := syncPathMonitor.Load()
	for pathIndex, expiringPathInfo := range mpq.paths {
		selectionKey := expiringPathInfo.path.Fingerprint()
		path := syncPathsData.APS.GetAppPath(selectionKey)
		if path.Fingerprint() != selectionKey {
			logger.Debug("Failed to refresh path, key does not match. Retrying later.",
				"Received", path.Fingerprint(), "Queried", selectionKey)
			logger.Trace("Path refresh details",
				"dst", mpq.paths[0].raddr.IA,
				"key", selectionKey,
				"path", expiringPathInfo.path.Interfaces())
		} else {
			freshExpTime := path.Expiry()
			if freshExpTime.After(mpq.paths[pathIndex].expiry) {
				mpq.paths[pathIndex].path = path

				// Update the path on the remote address
				tmpRaddr := mpq.paths[pathIndex].raddr.Copy()
				tmpRaddr.Path = path.Path()
				tmpRaddr.NextHop = path.OverlayNextHop()
				mpq.paths[pathIndex].raddr = tmpRaddr
				mpq.paths[pathIndex].path = path
				mpq.paths[pathIndex].revoked = false
				mpq.paths[pathIndex].expiry = mpq.paths[pathIndex].path.Expiry()

			} else {
				logger.Debug("Refreshed path does not have later expiry. Retrying later.")
				logger.Trace("Path refresh details",
					"dst", mpq.paths[0].raddr.IA,
					"key", selectionKey,
					"path", expiringPathInfo.path.Interfaces(),
					"currExp", mpq.paths[pathIndex].expiry,
					"freshExp", freshExpTime)
			}
		}
	}
}

// earliestPathExpiry computes the earliest expiration time of any path registered in mpq.
func (mpq *MPQuic) earliestPathExpiry() time.Time {
	ret := time.Now().Add(maxDuration)
	for _, pathInfo := range mpq.paths {
		if pathInfo.expiry.Before(ret) {
			ret = pathInfo.expiry
		}
	}
	return ret
}

// nextPathRefresh the time for the next scheduled path refresh, based on the
// expiration of the paths
func (mpq *MPQuic) nextPathRefresh() time.Time {
	expiry := mpq.earliestPathExpiry()
	randOffset := time.Duration(rand.Intn(10)) * time.Second
	return expiry.Add(-pathExpiryRefreshLeadTime + randOffset)
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
