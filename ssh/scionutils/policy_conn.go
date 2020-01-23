package scionutils

import (
	"context"
	"fmt"
	"github.com/scionproto/scion/go/lib/pathpol"
	"net"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

const (
	ErrNoPath     = "path not found"
	ErrInitPath   = "raw forwarding path offsets could not be initialized"
	ErrBadOverlay = "unable to extract next hop from sciond path entry"
)

type PathSelector interface {
	SelectPath(address snet.Addr) *spathmeta.AppPath
}

var _ net.PacketConn = (*policyConn)(nil)

// policyConn is a wrapper class around snet.SCIONConn
// policyConn overrides its WriteTo function, so that it chooses the path on which the packet is written based on
// a user-defined path policy and path selection mode defined in the field conf
// Subclasses must explicitly set pathSelectorFunc to their own SelectPath methods
// policyConn is not thread safe
type policyConn struct {
	snet.Conn
	conf         *PathAppConf
	pathSelector PathSelector
}
type defaultPathSelector struct {
	pathResolver pathmgr.Resolver
	localIA      addr.IA
}

func NewPolicyConn(c snet.Conn, resolver pathmgr.Resolver, localIA addr.IA) net.PacketConn {
	pc := &policyConn{
		Conn: c,
		pathSelector: &defaultPathSelector{
			pathResolver: resolver,
			localIA:      localIA,
		},
	}
	return pc
}

// WriteTo overrides snet.SCIONConn.WriteTo
// If the application calls Write instead of WriteTo, the logic defined here will be bypassed
func (c *policyConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	var err error
	remoteAddr := address.Copy()
	remoteAddr.NextHop, remoteAddr.Path, err = getSCIONPath(c.pathSelector.SelectPath(*address))
	if err != nil {
		return 0, common.NewBasicError("ConnWrapper: Path slection error: ", err)
	}
	return c.WriteToSCION(b, remoteAddr)
}

// SelectPath implements the path selection logic specified in PathAppConf
// Default behavior is arbitrary path selection
// Subclasses that wish to specialize path selection modes should override this function
func (s *defaultPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("default policyConn.. arbitrary path selection")
	pathSet := s.pathResolver.Query(context.Background(), s.localIA, address.IA, sciond.PathReqFlags{})
	appPath := pathSet.GetAppPath("")
	log.Trace(fmt.Sprintf("SELECTED PATH %s\n", appPath.Entry.Path))
	return appPath
}

// staticConnWraper is a subclass of policyConn which implements static path selection
// The connection uses the same path used in the first call to WriteTo for all subsequenet packets

type staticPathSelector struct {
	*defaultPathSelector
	staticPath *spathmeta.AppPath
	pathPolicy *pathpol.Policy
}

func NewStaticPolicyConn(c snet.Conn, resolver pathmgr.Resolver, localIA addr.IA, conf *PathAppConf) net.PacketConn {
	pc := &policyConn{Conn: c,
		pathSelector: &staticPathSelector{
			defaultPathSelector: &defaultPathSelector{
				pathResolver: resolver,
				localIA:      localIA,
			},
			pathPolicy: conf.Policy(),
		}}
	return pc
}

func (s *staticPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("staticPolicyConn.. selecting path")
	//if we're using a static path, query resolver only if this is the first call to write
	if s.staticPath == nil {
		log.Trace("querying resolver...")
		pathSet := s.pathResolver.QueryFilter(context.Background(), s.localIA, address.IA, s.pathPolicy)
		s.staticPath = pathSet.GetAppPath("")
	}
	log.Trace(fmt.Sprintf("Path exists: %s", s.staticPath))

	return s.staticPath
}

type roundRobinPathSelector struct {
	*defaultPathSelector
	paths        []*spathmeta.AppPath
	nextKeyIndex int
	pathPolicy   *pathpol.Policy
}

// roundrobinConnWraper is a subclass of policyConn which implements round-robin path selection
// For N arbitrarily ordered paths, the ith call for WriteTo uses the (i % N)th path

func NewRoundRobinPolicyConn(c snet.Conn, resolver pathmgr.Resolver, localIA addr.IA, conf *PathAppConf) net.PacketConn {
	pc := &policyConn{Conn: c,
		pathSelector: &roundRobinPathSelector{
			defaultPathSelector: &defaultPathSelector{
				pathResolver: resolver,
				localIA:      localIA,
			},
			nextKeyIndex: 0,
			pathPolicy:   conf.Policy(),
		}}
	return pc
}

func (c *roundRobinPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("roundRobinPolicyConn.. slecting path")
	// if there are no paths available, on the first call to WriteTo
	if len(c.paths) == 0 {
		pathMap := c.pathResolver.QueryFilter(context.Background(), c.localIA, address.IA, c.pathPolicy)
		for _, v := range pathMap {
			c.paths = append(c.paths, v)
		}
	}

	appPath := c.paths[c.nextKeyIndex]
	log.Trace(fmt.Sprintf("SELECTED PATH # %d: %s\n", c.nextKeyIndex, appPath.Entry.Path))
	c.nextKeyIndex = (c.nextKeyIndex + 1) % len(c.paths)
	return appPath

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
