package mpsquic

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qerr"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/tools/scmp/cmn"
)

var _ quic.Stream = (*monitoredStream)(nil)

type monitoredStream struct {
	quic.Stream
	underlayConn *MPQuic
}

// Write writes to a quic Stream while monitoring its approximate bandwidth
func (ms monitoredStream) Write(p []byte) (n int, err error) {
	//streamID := ms.Stream.StreamID()
	activeAtWriteStart := ms.underlayConn.active
	start := time.Now()
	n, err = ms.Stream.Write(p)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			logger.Error("Stream timeout", "err", err)
			return
		}
		if qErr, ok := err.(*qerr.QuicError); ok {
			if qErr.ErrorCode == qerr.NetworkIdleTimeout || qErr.ErrorCode == qerr.PeerGoingAway {
				// Remote went away
				logger.Error("Stream error", "err", err)
				return 0, qErr
			}
		}
		logger.Error("monitoredStream error", "err", err)
	}
	elapsed := time.Now().Sub(start)
	bandwidth := len(p) * 8 * 1e9 / int(elapsed)
	// Check if the path remained the same
	if ms.underlayConn.active == activeAtWriteStart {
		activeAtWriteStart.bw = bandwidth
	}
	return
}

// Read reads from a quic Stream while monitoring it
func (ms monitoredStream) Read(p []byte) (n int, err error) {
	n, err = ms.Stream.Read(p)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			logger.Error("Stream timeout", "err", err)
			return
		}
		if qErr, ok := err.(*qerr.QuicError); ok {
			if qErr.ErrorCode == qerr.NetworkIdleTimeout || qErr.ErrorCode == qerr.PeerGoingAway {
				// Remote went away
				logger.Error("Stream error", "err", err)
				return 0, qErr
			}
		}
		logger.Error("monitoredStream error", "err", err)
	}
	return
}

// monitor monitors the paths of mpq by sending SCMP messages at regular intervals and recording the replies in separate goroutines.
// It manages path expiration and path change decisions.
func (mpq *MPQuic) monitor() {
	cmn.Remote = *mpq.scionFlexConnection.raddr
	cmn.Local = *mpq.scionFlexConnection.laddr
	if cmn.Stats == nil {
		cmn.Stats = &cmn.ScmpStats{}
	}

	go mpq.sendSCMP()
	go mpq.rcvSCMP()

	go mpq.processRevocations()

	go mpq.managePaths()
}

// sendSCMP sends SCMP messages on all paths in mpq.
func (mpq *MPQuic) sendSCMP() {
	var seq uint16
	for {
		if mpq.dispConn == nil {
			break
		}

		for i := range mpq.paths {
			cmn.Remote = *mpq.paths[i].raddr
			id := uint64(i + 1)
			info := &scmp.InfoEcho{Id: id, Seq: seq}
			pkt := cmn.NewSCMPPkt(scmp.T_G_EchoRequest, info, nil)
			b := make(common.RawBytes, mpq.paths[i].path.Entry.Path.Mtu)
			nhAddr := cmn.NextHopAddr()

			nextPktTS := time.Now()
			cmn.UpdatePktTS(pkt, nextPktTS)
			// Serialize packet to internal buffer
			pktLen, err := hpkt.WriteScnPkt(pkt, b)
			if err != nil {
				logger.Error("Unable to serialize SCION packet.", "err", err)
				break
			}
			written, err := mpq.dispConn.WriteTo(b[:pktLen], nhAddr)
			if err != nil {
				logger.Error("Unable to write", "err", err)
				break
			} else if written != pktLen {
				logger.Error("Wrote incomplete message", "written", len(b), "expected", written)
				break
			}
			cmn.Stats.Sent += 1

			payload := pkt.Pld.(common.RawBytes)
			_, _ = info.Write(payload[scmp.MetaLen:])
			seq += 1
			logger.Trace("Sent SCMP packet", "len", pktLen, "payload", payload, "ID", info.Id)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// rcvSCMP receives SCMP messages and records the RTT for each path in mpq.
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
				if strings.Contains(err.Error(), "use of closed network connection") {
					logger.Info("Unable to read SCMP reply. Network down.")
					break
				}
				logger.Error("Unable to read SCMP reply.", "err", err)
				break
			}
		}
		now := time.Now()
		err = hpkt.ParseScnPkt(pkt, b[:pktLen])
		if err != nil {
			logger.Error("SCION packet parse", "err", err)
			continue
		}
		// Validate scmp packet
		scmpHdr, ok := pkt.L4.(*scmp.Hdr)
		if !ok {
			logger.Error("Not an SCMP header.", "type", common.TypeOf(pkt.L4))
			continue
		}
		scmpPld, ok := pkt.Pld.(*scmp.Payload)
		if !ok {
			logger.Error("Not an SCMP payload.", "type", common.TypeOf(pkt.Pld))
			continue
		}

		switch scmpPld.Info.(type) {
		case *scmp.InfoRevocation:
			pathKey, err := getSpathKey(*pkt.Path)
			if err != nil {
				logger.Error("Unable to map revocation to path key.", "err", err)
				continue
			}
			select {
				case revocationQ <- keyedRevocation{key: pathKey, revocationInfo: scmpPld.Info.(*scmp.InfoRevocation)}:
					logger.Trace("Processing scmp probe packet", "Action", "Revocation queued in revocationQ channel.")
				default:
					logger.Trace("Ignoring scmp probe packet", "Reason", "Revocation channel full.")
			}
		case *scmp.InfoEcho:
			cmn.Stats.Recv += 1
			// Calculate RTT
			rtt := now.Sub(scmpHdr.Time()).Round(time.Microsecond)
			scmpId := scmpPld.Info.(*scmp.InfoEcho).Id
			if scmpId-1 < uint64(len(mpq.paths)) {
				logger.Trace("Received SCMP packet", "len", pktLen, "ID", scmpId)
				mpq.paths[scmpId-1].rtt = rtt
			} else {
				logger.Error("Wrong InfoEcho Id.",  "id", scmpId)
			}
		default:
			logger.Error("Not an Info Echo.", "type", common.TypeOf(scmpPld.Info))
		}
	}
}

// getSpathKey returns a unique PathKey from a raw spath that can be used for map indexing.
func getSpathKey(path spath.Path) (pk *RawKey, err error) {
	h := sha256.New()
	err = binary.Write(h, common.Order, path.Raw)
	if err != nil {
		return nil, err
	}
	pkv := RawKey(h.Sum(nil))
	return &pkv, nil
}

// processRevocations processed entries on the revocationQ channel
func (mpq *MPQuic) processRevocations() {
	var rev keyedRevocation
	for {
		if mpq.dispConn == nil {
			break
		}
		rev = <-revocationQ
		logger.Trace("Processing revocation", "Action", "Retrieved queued revocation from revocationQ channel.")
		mpq.handleSCMPRevocation(rev.revocationInfo, rev.key)
	}
}

// Helper type for distinguishing keys on raw spaths.
type RawKey string

// String returns the string representation of the raw bytes of a RawKey (as obtained from the hash of a spath.Path.Raw)
func (pk RawKey) String() string {
	return common.RawBytes(pk).String()
}

// handleSCMPRevocation revocation handles explicit revocation notification of a link on a path being probed
// The active path is switched if the revocation expiration is in the future and was issued for an interface on the active path.
// If the revocation expiration is in the future, but for a backup path, the only the expiration time of the path is set to the current time.
func (mpq *MPQuic) handleSCMPRevocation(revocation *scmp.InfoRevocation, pk *RawKey) {
	signedRevInfo, err := path_mgmt.NewSignedRevInfoFromRaw(revocation.RawSRev)

	if err != nil {
		logger.Error("Unable to decode SignedRevInfo from SCMP InfoRevocation payload.", "err", err)
	}
	if err != nil {
		logger.Error("Failed to decode SCMP signed revocation Info.", "err", err)
	}
	ri, err := signedRevInfo.RevInfo()
	if err != nil {
		logger.Error("Failed to decode SCMP revocation Info.", "err", err)
	}

	// Revoke path from sciond
	mpq.network.PathResolver().RevokeRaw(context.TODO(), revocation.RawSRev)

	if ri.Expiration().After(time.Now()) {
		for i, pathInfo := range mpq.paths {
			if pathInfo.rawPathKey == *pk {
				mpq.paths[i].expiration = time.Now()
			}
		}
	} else {
		// Ignore expired revocations
		logger.Trace("Processing revocation", "Action", "Ignoring expired revocation.")
	}

	if pk == nil {
		logger.Error("Failed to process SCMP revocation.", "Invalid PathKey", pk)
		return
	}


	if *pk == mpq.active.rawPathKey {
		logger.Trace("Processing revocation", "Reason", "Revocation IS for active path.")
		err := mpq.switchMPConn(true, false)
		if err != nil {
			logger.Error("Failed to switch path after path revocation.", "err", err)
		}
	}
}

// refreshPaths requests sciond for updated paths
func (mpq *MPQuic) refreshPaths(resolver pathmgr.Resolver) {
	var filter *pathpol.Policy = nil
	sciondTimeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), sciondTimeout)
	defer cancel()
	syncPathMonitor, err := resolver.WatchFilter(ctx, mpq.network.IA(), mpq.paths[0].raddr.IA, filter)
	if err != nil {
		logger.Error("Failed to monitor paths.", "src", mpq.network.IA(), "dst", mpq.paths[0].raddr.IA,
			"filter", filter)
		syncPathMonitor = nil
	}

	if syncPathMonitor == nil {
		return
	}

	syncPathsData := syncPathMonitor.Load()
	for pathIndex, expiringPathInfo := range mpq.paths {
		selectionKey := expiringPathInfo.path.Key()
		appPath := syncPathsData.APS.GetAppPath(selectionKey)
		if appPath.Key() != selectionKey {
			logger.Debug("Failed to refresh path, key does not match. Retrying later.", "Received", appPath.Key(), "Queried", selectionKey)
			logger.Trace("Path refresh details",
				"src", mpq.network.IA(),
				"dst", mpq.paths[0].raddr.IA,
				"key", selectionKey,
				"path", expiringPathInfo.path.Entry.Path.Interfaces,
				"filter", filter)
		} else {
			freshExpTime := appPath.Entry.Path.Expiry()
			if freshExpTime.After(mpq.paths[pathIndex].expiration) {
				mpq.paths[pathIndex].path = *appPath

				// Update the path on the remote address
				newPath := spath.New(appPath.Entry.Path.FwdPath)
				_ = newPath.InitOffsets()
				tmpRaddr := mpq.paths[pathIndex].raddr.Copy()
				tmpRaddr.Path = newPath
				tmpRaddr.NextHop, _ = appPath.Entry.HostInfo.Overlay()
				mpq.paths[pathIndex].raddr = tmpRaddr
				mpq.paths[pathIndex].path = spathmeta.AppPath{appPath.Entry}
				mpq.paths[pathIndex].expiration = mpq.paths[pathIndex].path.Entry.Path.Expiry()
			} else {
				logger.Debug("Refreshed path does not have later expiry. Retrying later.")
				logger.Trace("Path refresh details",
					"src", mpq.network.IA(),
					"dst", mpq.paths[0].raddr.IA,
					"key", selectionKey,
					"path", expiringPathInfo.path.Entry.Path.Interfaces,
					"filter", filter,
					"currExp", mpq.paths[pathIndex].expiration,
					"freshExp", freshExpTime)
			}
		}
	}
}

// earliestPathExpiry computes the earliest expiration time of any path registered in mpq.
func (mpq *MPQuic) earliestPathExpiry() (ret time.Time) {
	ret = time.Now().Add(maxDuration)
	for _, pathInfo := range mpq.paths {
		if pathInfo.expiration.Before(ret) {
			ret = pathInfo.expiration
		}
	}
	return
}

// managePaths evaluates every 5 seconds if a path is about to expire and if there is a better path to switch to.
func (mpq *MPQuic) managePaths() {
	lastUpdate := time.Now()
	pr := mpq.network.PathResolver()
	// Get initial expiration time of all paths
	for i, pathInfo := range mpq.paths {
		mpq.paths[i].expiration = pathInfo.path.Entry.Path.Expiry()
	}

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
		err := mpq.switchMPConn(false, true)
		if err != nil {
			logger.Error("Failed to switch path.", "err", err)
		}
		lastUpdate = time.Now()
	}
}
