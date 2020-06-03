package mpsquic

import (
	"context"
	"time"

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
	default:
		logger.Info("Ignoring scmp packet", "cause", "Revocation channel full.")
	}
}

// handleSCMPRevocation handles explicit revocation notification of a link on a path being probed
// The active path is switched if the revocation expiration is in the future and was issued for an interface on the active path.
// If the revocation expiration is in the future, but for a backup path, then only the expiration time of the path is set to the current time.
func (mpq *MPQuic) handleRevocation(sRevInfo *path_mgmt.SignedRevInfo) {

	// Revoke path from sciond
	mpq.pathResolver.Revoke(context.TODO(), sRevInfo)

	// Revoke from our own state
	revInfo, err := sRevInfo.RevInfo()
	if err != nil {
		logger.Error("Failed to decode signed revocation info", "err", err)
	}

	revokedInterface := sciond.PathInterface{RawIsdas: revInfo.RawIsdas,
		IfID: common.IFIDType(revInfo.IfID)}

	if revInfo.Active() == nil {
		for _, pathInfo := range mpq.paths {
			if matches(pathInfo.path, revokedInterface) {
				pathInfo.expiry = time.Now()
			}
		}
		if matches(mpq.active.path, revokedInterface) {
			logger.Trace("Processing revocation", "reason", "Revocation IS for active path.")
			err := mpq.switchMPConn(true, false)
			if err != nil {
				logger.Error("Failed to switch path after path revocation.", "err", err)
			}
		}
	} else {
		// Ignore expired revocations
		logger.Trace("Processing revocation", "action", "Ignoring expired revocation.")
	}
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
