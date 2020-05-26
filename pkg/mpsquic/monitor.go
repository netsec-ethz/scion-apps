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
	"github.com/netsec-ethz/scion-apps/pkg/appnet"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spkt"
)

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

// monitor monitors the paths of mpq by sending SCMP messages at regular intervals and recording the replies in separate goroutines.
// It manages path expiration and path change decisions.
func (mpq *MPQuic) monitor() {
	var appID uint64 = rand.Uint64()
	go mpq.sendSCMP(appID)
	go mpq.rcvSCMP(appID)

	go mpq.processRevocations()

	go mpq.managePaths()
}

// sendSCMP sends SCMP messages on all paths in mpq.
func (mpq *MPQuic) sendSCMP(appID uint64) {
	var seq uint16
	for {
		if mpq.dispConn == nil {
			break
		}

		localIA := appnet.DefNetwork().IA
		localIP := mpq.flexConn.Conn.LocalAddr().(*net.UDPAddr).IP
		laddr := &snet.UDPAddr{IA: localIA, Host: &net.UDPAddr{IP: localIP}}
		for i := range mpq.paths {
			raddr := mpq.paths[i].raddr
			scmpID := appID + uint64(i)
			pkt := newSCMPPkt(
				laddr,
				raddr,
				scmp.T_G_EchoRequest,
				&scmp.InfoEcho{Id: scmpID, Seq: seq},
			)
			_ = writeTo(mpq.dispConn, pkt, raddr.NextHop)

			seq += 1
			logger.Trace("Sent SCMP packet", "Id", scmpID, "Seq", seq)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func newSCMPPkt(src, dst *snet.UDPAddr, t scmp.Type, info scmp.Info) *snet.Packet {
	const bufSize = 4 * 1024
	buf := make(snet.Bytes, bufSize) // should be reused, but not critical right now

	scmpMeta := scmp.Meta{InfoLen: uint8(info.Len() / common.LineLen)}
	pld := make(common.RawBytes, scmp.MetaLen+info.Len())
	err := scmpMeta.Write(pld)
	if err != nil {
		panic(err)
	}
	_, err = info.Write(pld[scmp.MetaLen:])
	if err != nil {
		panic(err)
	}
	scmpHdr := scmp.NewHdr(scmp.ClassType{Class: scmp.C_General, Type: t}, len(pld))
	return &snet.Packet{
		Bytes: buf,
		PacketInfo: snet.PacketInfo{
			Source:      snet.SCIONAddress{IA: src.IA, Host: addr.HostFromIP(src.Host.IP)},
			Destination: snet.SCIONAddress{IA: dst.IA, Host: addr.HostFromIP(dst.Host.IP)},
			Path:        dst.Path,
			L4Header:    scmpHdr,
			Payload:     pld,
		},
	}
}

// writeTo is a (shortened) copy of SCIONPacketConn.WriteTo.
// TODO: find out whether we can just migrate this to use SCIONPacketConn directly. How does this
// change receiving SCMPs?
func writeTo(dispConn net.PacketConn, pkt *snet.Packet, ov *net.UDPAddr) error {
	scnPkt := &spkt.ScnPkt{
		DstIA:   pkt.Destination.IA,
		SrcIA:   pkt.Source.IA,
		DstHost: pkt.Destination.Host,
		SrcHost: pkt.Source.Host,
		Path:    pkt.Path,
		L4:      pkt.L4Header,
		Pld:     pkt.Payload,
	}
	pkt.Prepare()
	n, err := hpkt.WriteScnPkt(scnPkt, common.RawBytes(pkt.Bytes))
	if err != nil {
		return common.NewBasicError("Unable to serialize SCION packet", err)
	}
	pkt.Bytes = pkt.Bytes[:n]
	// Send message
	_, err = dispConn.WriteTo(pkt.Bytes, ov)
	if err != nil {
		return common.NewBasicError("Reliable socket write error", err)
	}
	return nil
}

// rcvSCMP receives SCMP messages and records the RTT for each path in mpq.
func (mpq *MPQuic) rcvSCMP(appID uint64) {
	for {
		if mpq.dispConn == nil {
			break
		}

		pkt := &spkt.ScnPkt{}
		b := make(common.RawBytes, 1500)

		pktLen, _, err := mpq.dispConn.ReadFrom(b)
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
			pathKey, err := getSpathKey(pkt.Path)
			if err != nil {
				logger.Error("Unable to map revocation to path key.", "err", err)
				continue
			}
			select {
			case mpq.revocationQ <- keyedRevocation{key: pathKey, revocationInfo: scmpPld.Info.(*scmp.InfoRevocation)}:
				logger.Trace("Processing scmp probe packet", "Action", "Revocation queued in revocationQ channel.")
			default:
				logger.Trace("Ignoring scmp probe packet", "Reason", "Revocation channel full.")
			}
		case *scmp.InfoEcho:
			// Calculate RTT
			rtt := now.Sub(scmpHdr.Time()).Round(time.Microsecond)
			scmpID := scmpPld.Info.(*scmp.InfoEcho).Id
			if scmpID >= appID && scmpID < appID+uint64(len(mpq.paths)) {
				logger.Trace("Received SCMP packet", "len", pktLen, "ID", scmpID-appID)
				mpq.paths[scmpID-appID].rtt = rtt
			} else {
				logger.Error("Wrong InfoEcho Id.", "id", scmpID-appID, "appID", appID)
			}
		default:
			logger.Error("Not an Info Echo.", "type", common.TypeOf(scmpPld.Info))
		}
	}
}

// getSpathKey returns a unique PathKey from a raw spath that can be used for map indexing.
func getSpathKey(path *spath.Path) (pk RawKey, err error) {
	h := sha256.New()
	err = binary.Write(h, common.Order, path.Raw)
	if err != nil {
		return RawKey(""), err
	}
	return RawKey(h.Sum(nil)), nil
}

// processRevocations processed entries on the revocationQ channel
func (mpq *MPQuic) processRevocations() {
	var rev keyedRevocation
	for {
		if mpq.dispConn == nil {
			break
		}
		rev = <-mpq.revocationQ
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

// handleSCMPRevocation handles explicit revocation notification of a link on a path being probed
// The active path is switched if the revocation expiration is in the future and was issued for an interface on the active path.
// If the revocation expiration is in the future, but for a backup path, then only the expiration time of the path is set to the current time.
func (mpq *MPQuic) handleSCMPRevocation(revocation *scmp.InfoRevocation, pk RawKey) {
	signedRevInfo, err := path_mgmt.NewSignedRevInfoFromRaw(revocation.RawSRev)

	if err != nil {
		logger.Error("Unable to decode SignedRevInfo from SCMP InfoRevocation payload.", "err", err)
	}
	ri, err := signedRevInfo.RevInfo()
	if err != nil {
		logger.Error("Failed to decode SCMP signed revocation Info.", "err", err)
	}

	// Revoke path from sciond
	mpq.pathResolver.RevokeRaw(context.TODO(), revocation.RawSRev)

	if ri.Expiration().After(time.Now()) {
		for i, pathInfo := range mpq.paths {
			if pathInfo.rawPathKey == pk {
				mpq.paths[i].expiry = time.Now()
			}
		}
	} else {
		// Ignore expired revocations
		logger.Trace("Processing revocation", "Action", "Ignoring expired revocation.")
	}

	if pk == mpq.active.rawPathKey {
		logger.Trace("Processing revocation", "Reason", "Revocation IS for active path.")
		err := mpq.switchMPConn(true, false)
		if err != nil {
			logger.Error("Failed to switch path after path revocation.", "err", err)
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
