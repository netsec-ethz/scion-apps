package scionutils

import (
	"context"
	"fmt"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"net"
)

const (
	ErrNoPath     = "path not found"
	ErrInitPath   = "raw forwarding path offsets could not be initialized"
	ErrBadOverlay = "unable to extract next hop from sciond path entry"
)

type PathSelector interface {
	SelectPath(address snet.Addr) (*overlay.OverlayAddr, *spath.Path, error)
}

type pathConverter func (appPath *spathmeta.AppPath) (*overlay.OverlayAddr, *spath.Path, error)

var _ net.PacketConn = (*policyConn)(nil)

// policyConn is a wrapper class around snet.SCIONConn
// policyConn overrides its WriteTo function, so that it chooses the path on which the packet is written based on
// a user-defined path policy and path selection mode defined in the field conf
// Subclasses must explicitly set pathSelectorFunc to their own SelectPath methods
// policyConn is not thread safe
type policyConn struct {
	snet.Conn
	conf             *PathAppConf
	pathResolver     pathmgr.Resolver
	localIA          addr.IA
	pathSelector PathSelector
	pathConverter pathConverter
}

func NewPolicyConn(c snet.Conn, conf *PathAppConf) *policyConn {
	pc := &policyConn{
		Conn:         c,
		conf:         conf,}
	pc.pathSelector = pc
	pc.pathConverter = getSCIONPath
	return pc
}

// WriteTo overrides snet.SCIONConn.WriteTo
// If the application calls Write instead of WriteTo, the logic defined here will be bypassed
func (c policyConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	var err error
	remoteAddr := address.Copy()
	remoteAddr.NextHop, remoteAddr.Path, err = c.pathSelector.SelectPath(*address)
	if err != nil {
		return 0, common.NewBasicError("ConnWrapper: Path slection error: ", err)
	}
	return c.WriteToSCION(b, remoteAddr)
}

// SelectPath implements the path selection logic specified in PathAppConf
// Default behavior is arbitrary path selection
// Subclasses that wish to specialize path selection modes should override this function
func (c *policyConn) SelectPath(address snet.Addr) (*overlay.OverlayAddr, *spath.Path, error) {
	log.Trace("default policyConn.. arbitrary path selection")
	pathSet := c.pathResolver.Query(context.Background(), c.localIA, address.IA, sciond.PathReqFlags{})
	appPath := pathSet.GetAppPath("")
	nextHop, path, err := getSCIONPath(appPath)
	if err != nil {
		return nil, nil, common.NewBasicError(fmt.Sprintf("Conn Wrapper: Arbitrary : error getting SCION path "+
			"between client %s and server %s", c.localIA, address.IA), err)
	}
	log.Trace(fmt.Sprintf("SELECTED PATH %s\n", appPath.Entry.Path))
	return nextHop, path, nil
}

// staticConnWraper is a subclass of policyConn which implements static path selection
// The connection uses the same path used in the first call to WriteTo for all subsequenet packets
type staticPolicyConn struct {
	policyConn
	staticPath    *spath.Path
	staticNextHop *overlay.OverlayAddr
}

func NewStaticPolicyConn(c policyConn) *staticPolicyConn {
	pc := &staticPolicyConn{policyConn: c}
	pc.pathSelector = pc
	return pc
}

func (c *staticPolicyConn) SelectPath(address snet.Addr) (*overlay.OverlayAddr, *spath.Path, error) {
	log.Trace("staticPolicyConn.. selecting path")
	//if we're using a static path, query resolver only if this is the first call to write
	if c.staticNextHop == nil && c.staticPath == nil {
		log.Trace("querying resolver...")
		pathSet := c.pathResolver.QueryFilter(context.Background(), c.localIA, address.IA, c.conf.Policy())
		appPath := pathSet.GetAppPath("")
		nextHop, path, err := getSCIONPath(appPath)
		log.Trace(fmt.Sprintf("SELECTED PATH: %s\n", appPath.Entry.Path))
		if err != nil {
			return nil, nil, common.NewBasicError(fmt.Sprintf("staticPolicyConn: error getting SCION path "+
				"between client %s and server %s", c.localIA, address.IA), err)
		}
		c.staticPath, c.staticNextHop = path, nextHop
	} else if c.staticNextHop != nil && c.staticPath == nil || c.staticNextHop == nil && c.staticPath != nil {
		return nil, nil, common.NewBasicError("staticPolicyConn:"+
			"Next hop and path must both be either defined or undefined", nil)
	}
	log.Trace(fmt.Sprintf("Path exists: %v", c.staticPath))

	return c.staticNextHop, c.staticPath, nil

}

// roundrobinConnWraper is a subclass of policyConn which implements round-robin path selection
// For N arbitrarily ordered paths, the ith call for WriteTo uses the (i % N)th path
type roundRobinPolicyConn struct {
	policyConn
	paths        []*spathmeta.AppPath
	nextKeyIndex int
}

func NewRoundRobinPolicyConn(c policyConn) *roundRobinPolicyConn {
	pc := &roundRobinPolicyConn{policyConn: c}
	pc.pathSelector = pc
	return pc
}

func (c *roundRobinPolicyConn) SelectPath(address snet.Addr) (*overlay.OverlayAddr, *spath.Path, error) {
	log.Trace("roundRobinPolicyConn.. slecting path")
	// if there are no paths available, on the first call to WriteTo
	if len(c.paths) == 0 {
		pathMap := c.pathResolver.QueryFilter(context.Background(), c.localIA, address.IA, c.conf.Policy())
		for _, v := range pathMap {
			c.paths = append(c.paths, v)
		}
	}

	appPath := c.paths[c.nextKeyIndex]
	log.Trace(fmt.Sprintf("SELECTED PATH # %d: %s\n", c.nextKeyIndex, appPath.Entry.Path))
	c.nextKeyIndex = (c.nextKeyIndex + 1) % len(c.paths)
	nextHop, path, err := c.pathConverter(appPath)
	if err != nil {
		return nil, nil, common.NewBasicError(fmt.Sprintf("roundRobinPolicyConn: error getting SCION path"+
			" between client %s and server %s", c.localIA, address.IA), err)
	}
	return nextHop, path, nil

}


func getSCIONPath(appPath *spathmeta.AppPath) (*overlay.OverlayAddr, *spath.Path, error) {
	if appPath == nil {
		return nil, nil, common.NewBasicError(ErrNoPath, nil)
	}
	path := &spath.Path{Raw: appPath.Entry.Path.FwdPath}
	if err := path.InitOffsets(); err != nil {
		return nil, nil, common.NewBasicError(ErrInitPath, err)
	}
	overlayAddr, err := appPath.Entry.HostInfo.Overlay()
	if err != nil {
		return nil, nil, common.NewBasicError(ErrBadOverlay, err)
	}
	return overlayAddr, path, nil

}
