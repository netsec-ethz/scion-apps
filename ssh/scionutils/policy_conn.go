package scionutils

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/scionproto/scion/go/lib/pathpol"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

// error strings
const (
	ErrNoPath     = "path not found"
	ErrInitPath   = "raw forwarding path offsets could not be initialized"
	ErrBadOverlay = "unable to extract next hop from sciond path entry"
)

// PathSelector selects a path for a given address.
type PathSelector interface {
	SelectPath(address snet.Addr) *spathmeta.AppPath
}

var _ PathSelector = (*defaultPathSelector)(nil)

type defaultPathSelector struct {
	pathResolver pathmgr.Resolver
	localIA      addr.IA
}

// SelectPath implements the path selection logic specified in PathAppConf
// Default behavior is arbitrary path selection
// Subclasses that wish to specialize path selection modes should override this function
func (s *defaultPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("default policyConn.. arbitrary path selection")
	pathSet := s.pathResolver.Query(context.Background(), s.localIA, address.IA,
		sciond.PathReqFlags{})
	appPath := pathSet.GetAppPath("")
	log.Trace(fmt.Sprintf("SELECTED PATH %s\n", appPath.Entry.Path))
	return appPath
}

// staticPathSelector is a subclass of policyConn which implements static path selection
// The connection uses the same path used in the first call to WriteTo for all subsequenet packets
type staticPathSelector struct {
	defaultPathSelector
	staticPath *spathmeta.AppPath
	pathPolicy *pathpol.Policy
	initOnce   sync.Once
}

func (s *staticPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("staticPolicyConn.. selecting path")
	s.initOnce.Do(func() {
		log.Trace("staticPathSelector, initializing paths")
		pathSet := s.pathResolver.QueryFilter(context.Background(), s.localIA, address.IA,
			s.pathPolicy)
		s.staticPath = pathSet.GetAppPath("")
	})
	log.Trace(fmt.Sprintf("Path exists: %s", s.staticPath))

	return s.staticPath
}

// roundrobinPathSelector is a subclass of policyConn which implements round-robin path selection
// For N arbitrarily ordered paths, the ith call for WriteTo uses the (i % N)th path
type roundRobinPathSelector struct {
	defaultPathSelector
	paths        []*spathmeta.AppPath
	nextKeyIndex int
	pathPolicy   *pathpol.Policy
	initOnce     sync.Once
}

func (s *roundRobinPathSelector) SelectPath(address snet.Addr) *spathmeta.AppPath {
	log.Trace("roundRobinPolicyConn.. slecting path")
	s.initOnce.Do(func() {
		pathMap := s.pathResolver.QueryFilter(context.Background(), s.localIA, address.IA,
			s.pathPolicy)
		for _, v := range pathMap {
			s.paths = append(s.paths, v)
		}
	})

	appPath := s.paths[s.nextKeyIndex]
	log.Trace(fmt.Sprintf("SELECTED PATH # %d: %s\n", s.nextKeyIndex, appPath.Entry.Path))
	s.nextKeyIndex = (s.nextKeyIndex + 1) % len(s.paths)
	return appPath
}

// policyConn is a wrapper class around snet.SCIONConn that overrides its WriteTo function,
// so that it chooses the path on which the packet is written.
type policyConn struct {
	snet.Conn
	pathSelector PathSelector
}

var _ net.PacketConn = (*policyConn)(nil)

// NewPolicyConn constructs a PolicyConn specified in the PathAppConf argument.
func NewPolicyConn(conf *PathAppConf, c snet.Conn, resolver pathmgr.Resolver,
	localIA addr.IA) net.PacketConn {

	var pathSel PathSelector
	pathSel = &defaultPathSelector{
		pathResolver: resolver,
		localIA:      localIA,
	}
	switch conf.PathSelection() {
	case Static:
		pathSel = &staticPathSelector{
			defaultPathSelector: *pathSel.(*defaultPathSelector),
			pathPolicy:          conf.Policy(),
		}
	case RoundRobin:
		pathSel = &roundRobinPathSelector{
			defaultPathSelector: *pathSel.(*defaultPathSelector),
			pathPolicy:          conf.Policy(),
		}
	}
	return &policyConn{
		Conn:         c,
		pathSelector: pathSel,
	}
}

// WriteTo overrides snet.SCIONConn.WriteTo
// If the application calls Write instead of WriteTo, the logic defined here will be bypassed
func (c *policyConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	remoteAddr, err := prepareSCIONAddress(address, c.pathSelector.SelectPath(*address))
	if err != nil {
		return 0, common.NewBasicError("ConnWrapper: Path slection error: ", err)
	}
	return c.WriteToSCION(b, remoteAddr)
}

func prepareSCIONAddress(address *snet.Addr, appPath *spathmeta.AppPath) (*snet.Addr, error) {
	if appPath == nil {
		return nil, common.NewBasicError(ErrNoPath, nil)
	}
	path := &spath.Path{Raw: appPath.Entry.Path.FwdPath}
	if err := path.InitOffsets(); err != nil {
		return nil, common.NewBasicError(ErrInitPath, err)
	}
	overlayAddr, err := appPath.Entry.HostInfo.Overlay()
	if err != nil {
		return nil, common.NewBasicError(ErrBadOverlay, err)
	}
	readyAddress := address.Copy()
	readyAddress.Path = path
	readyAddress.NextHop = overlayAddr
	return readyAddress, nil
}
