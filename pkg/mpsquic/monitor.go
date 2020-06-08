package mpsquic

import (
	"context"
	"math/rand"
	"runtime"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"

	"github.com/scionproto/scion/go/lib/pathmgr"
)

const echoRequestInterval = 200 * time.Millisecond

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
		activeAtWriteStart.bw = bandwidth
	}
	return
}

// monitor monitors the paths of mpq by sending SCMP messages at regular
// intervals and recording the replies in separate goroutines.
// It manages path expiration and path change decisions.
func (mpq *MPQuic) monitor() {
	var appID uint64 = rand.Uint64()
	go mpq.sendPings(appID)
	go mpq.rcvPings(appID)
	go mpq.processRevocations()
	go mpq.managePaths()
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
		logger.Trace("Received SCMP echo reply", "id", scmpID)
		pathID := scmpID - appID
		mpq.paths[pathID].rtt = reply.RTT
	} else {
		logger.Error("Unexpected SCMP echo reply", "id", scmpID, "appID", appID)
	}
}

// processRevocations processed entries on the revocationQ channel
func (mpq *MPQuic) processRevocations() {
	for {
		select {
		case <-mpq.stop:
			break
		case rev := <-mpq.revocationQ:
			logger.Trace("Processing revocation", "action", "Retrieved queued revocation from revocationQ channel.")
			mpq.handleRevocation(rev)
		}
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
func (mpq *MPQuic) earliestPathExpiry() (ret time.Time) {
	ret = time.Now().Add(maxDuration)
	for _, pathInfo := range mpq.paths {
		if pathInfo.expiry.Before(ret) {
			ret = pathInfo.expiry
		}
	}
	return
}

// managePaths evaluates every 5 seconds if a path is about to expire and if
// there is a better path to switch to.
func (mpq *MPQuic) managePaths() {
	lastUpdate := time.Now()
	// Busy wait until we have at least measurements on two paths
	for {
		var measuredPaths int
		for _, pathInfo := range mpq.paths {
			if pathInfo.rtt != maxDuration {
				measuredPaths += 1
			}
		}
		if measuredPaths > 1 {
			break
		}
		runtime.Gosched()
	}

	// Make a (voluntary) path change decision to increase performance at most once per 5 seconds
	// Use the time in between to refresh the path information if required
	var maxFlap time.Duration = 5 * time.Second
	for {
		select {
		case <-mpq.stop:
			break
		default:
		}

		earliesExp := mpq.earliestPathExpiry()
		// Refresh the paths if one of them expires in less than 10 minutes
		if time.Until(earliesExp) < (10*time.Minute - time.Duration(rand.Intn(10))*time.Second) {
			mpq.refreshPaths(mpq.pathResolver)
		}

		sinceLastUpdate := time.Since(lastUpdate)
		time.Sleep(maxFlap - sinceLastUpdate) // Failing paths are handled separately / faster by handleSCMPRevocation
		err := mpq.switchMPConn(false, true)
		if err != nil {
			logger.Error("Failed to switch path.", "err", err)
		}
		lastUpdate = time.Now()
	}
}
