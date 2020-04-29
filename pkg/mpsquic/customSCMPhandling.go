package mpsquic

import (
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

type scmpHandler struct {
	// pathResolver manages revocations received via SCMP. If nil, nothing is informed.
	pathResolver pathmgr.Resolver
	revocationQ  chan keyedRevocation
}

// Handle handles the SCMP header information and triggers SCMP specific handlers
func (h *scmpHandler) Handle(pkt *snet.SCIONPacket) error {
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

// handleSCMPRev handles SCMP revocations and adds keyedRevocation to the revocationQ channel if the revocation parses
func (h *scmpHandler) handleSCMPRev(hdr *scmp.Hdr, pkt *snet.SCIONPacket) error {
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
	if err != nil{
		return common.NewBasicError("Unable to reverse path from packet with SCMP revocation", err)
	}
	pathKey, err := getSpathKey(*rpath)
	if err != nil || pathKey == nil{
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

// initNetworkWithPRCustomSCMPHandler user the default snet DefaultPacketDispatcherService, but
// with a custom SCMP handler
func initNetworkWithPRCustomSCMPHandler(ia addr.IA, sciondPath string, dispatcher reliable.DispatcherService) error {
	pathResolver, err := getResolver(sciondPath)
	if err != nil {
		return err
	}
	network := snet.NewCustomNetworkWithPR(ia,
		&snet.DefaultPacketDispatcherService{
			Dispatcher: dispatcher,
			SCMPHandler: &scmpHandler{
				pathResolver: pathResolver,
				revocationQ:  revocationQ,
			},
		},
		pathResolver,
	)
	return snet.InitWithNetwork(network)
}

// getResolver builds a default resolver for mpsquic internals.
func getResolver(sciondPath string) (pathmgr.Resolver, error) {
	var pathResolver pathmgr.Resolver
	if sciondPath != "" {
		sciondConn, err := sciond.NewService(sciondPath, true).Connect()
		if err != nil {
			return nil, common.NewBasicError("Unable to initialize SCIOND service", err)
		}
		pathResolver = pathmgr.New(
			sciondConn,
			pathmgr.Timers{
				NormalRefire: time.Minute,
				ErrorRefire:  3 * time.Second,
			},
		)
	}
	return pathResolver, nil
}
