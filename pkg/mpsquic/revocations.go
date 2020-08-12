package mpsquic

import (
	"context"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ snet.RevocationHandler = (*revocationHandler)(nil)

type revocationHandler struct {
	revocationQ chan *path_mgmt.SignedRevInfo
}

func (h *revocationHandler) RevokeRaw(_ context.Context, rawSRevInfo common.RawBytes) {
	sRevInfo, err := path_mgmt.NewSignedRevInfoFromRaw(rawSRevInfo)
	if err != nil {
		logger.Error("Revocation failed, unable to parse signed revocation info",
			"raw", rawSRevInfo, "err", err)
		return
	}
	select {
	case h.revocationQ <- sRevInfo:
		logger.Info("Enqueued revocation", "revInfo", sRevInfo)
	default:
		logger.Info("Ignoring scmp packet", "cause", "Revocation channel full.")
	}
}

// handleSCMPRevocation handles explicit revocation notification of a link on a
// path that is either being probed or actively used for the data stream.
// Returns true iff the currently active path was revoked.
func (mpq *MPQuic) handleRevocation(sRevInfo *path_mgmt.SignedRevInfo) bool {

	// Revoke path from sciond
	_, _ = appnet.DefNetwork().Sciond.RevNotification(context.TODO(), sRevInfo)

	// Revoke from our own state
	revInfo, err := sRevInfo.RevInfo()
	if err != nil {
		logger.Error("Failed to decode signed revocation info", "err", err)
	}

	revokedInterface := sciond.PathInterface{RawIsdas: revInfo.RawIsdas,
		IfID: common.IFIDType(revInfo.IfID)}

	activePathRevoked := false
	if revInfo.Active() == nil {
		for i, pathInfo := range mpq.paths {
			if matches(pathInfo.path, revokedInterface) {
				pathInfo.revoked = true
				if i == mpq.active {
					activePathRevoked = true
				}
			}
		}
	} else {
		// Ignore expired revocations
		logger.Trace("Processing revocation", "action", "Ignoring expired revocation.")
	}
	return activePathRevoked
}

// matches returns true if the path contains the interface described by ia/ifID
func matches(path snet.Path, predicatePI sciond.PathInterface) bool {
	for _, pi := range path.Interfaces() {
		if pi.IA().Equal(predicatePI.IA()) && pi.ID() == predicatePI.ID() {
			return true
		}
	}
	return false
}
