package mpsquic

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
)

type scmpHandler struct {
	revocationQ chan keyedRevocation
}

// Handle handles the SCMP header information and triggers SCMP specific handlers
func (h *scmpHandler) Handle(pkt *snet.Packet) error {
	hdr, ok := pkt.L4Header.(*scmp.Hdr)
	if !ok {
		return common.NewBasicError("scmp handler invoked with non-scmp packet", nil, "pkt", pkt)
	}

	// Only handle revocations for now
	if hdr.Class == scmp.C_Path && hdr.Type == scmp.T_P_RevokedIF {
		return h.handleSCMPRev(hdr, pkt)
	}
	logger.Info("Ignoring scmp packet", "hdr", hdr, "src", pkt.Source)
	return nil
}

// handleSCMPRev handles SCMP revocations and adds keyedRevocation to the
// revocationQ channel if the revocation parses
func (h *scmpHandler) handleSCMPRev(hdr *scmp.Hdr, pkt *snet.Packet) error {
	scmpPayload, ok := pkt.Payload.(*scmp.Payload)
	if !ok {
		return common.NewBasicError("Unable to type assert payload to SCMP payload", nil,
			"type", common.TypeOf(pkt.Payload))
	}
	info, ok := scmpPayload.Info.(*scmp.InfoRevocation)
	if !ok {
		return common.NewBasicError("Unable to type assert SCMP Info to SCMP Revocation Info", nil,
			"type", common.TypeOf(scmpPayload.Info))
	}
	logger.Trace("Received SCMP revocation",
		"header", hdr.String(),
		"payload", scmpPayload.String(),
		"src", pkt.Source)
	rpath := pkt.Path
	err := rpath.Reverse()
	if err != nil {
		return common.NewBasicError("Unable to reverse path from packet with SCMP revocation", err)
	}
	pathKey, err := getSpathKey(rpath)
	if err != nil {
		return common.NewBasicError("Unable to extract path key from packet with SCMP revocation", err)
	}
	select {
	case h.revocationQ <- keyedRevocation{key: pathKey, revocationInfo: info}:
	default:
		logger.Info("Ignoring scmp packet", "cause", "Revocation channel full.")
	}
	// Path revocation has been triggered
	return nil
}
