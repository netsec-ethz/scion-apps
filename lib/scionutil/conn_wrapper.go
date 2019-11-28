package scionutil

import (
	"context"
	"fmt"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/overlay"
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
var _ net.PacketConn = (*connWrapper)(nil)
var _ net.Conn = (*connWrapper) (nil)
var _ snet.Conn = (*connWrapper) (nil)

// connWrapper is a wrapper class around snet.SCIONConn
// connWrapper overrides its WriteTo function, so that it chooses the path on which the packet is written based on
// a user-defined path policy and path selection mode defined in the field conf
type connWrapper struct {
	*snet.SCIONConn
	conf *AppConf
	pathMap spathmeta.AppPathSet
	pathKeys []spathmeta.PathKey
	nextKeyIndex int
}

func NewConnWrapper (c snet.Conn, conf *AppConf) *connWrapper {
	return &connWrapper{SCIONConn: c.(*snet.SCIONConn), conf: conf}
}

func (c *connWrapper) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	resolver := snet.DefNetwork.PathResolver()
	localIA := c.LocalSnetAddr().IA
	remoteAddr := address.Copy()
	var appPath *spathmeta.AppPath
	var nextHop *overlay.OverlayAddr
	var path *spath.Path
	var err error

	switch c.conf.PathSelection() {
	case Static:
		log.Trace("ConnWrapper: STATIC path selection")
		staticNextHop , staticPath := c.conf.GetStaticPath()
		//if we're using a static path, query resolver only if this is the first call to write
		if  staticNextHop == nil && staticPath == nil {
			log.Trace("Querying Resolver - First Time")
			pathSet := resolver.QueryFilter(context.Background(), localIA, address.IA, c.conf.Policy())
			appPath = pathSet.GetAppPath("")
			nextHop, path, err = getSCIONPath(appPath)
			if err != nil {
				return 0, common.NewBasicError(fmt.Sprintf("Conn Wrapper: Static : error getting SCION path " +
					"between client %v and server %v", localIA.String(), address.IA.String()), err)
			}
			c.conf.SetStaticPath(nextHop, path)
		} else if staticNextHop != nil && staticPath != nil {
			nextHop, path = staticNextHop, staticPath
			log.Trace("Found old path: %v", staticPath)
		} else {
			return 0, common.NewBasicError("Next hop and path must both be either defined or undefined", nil)
		}
	case Arbitrary:
		log.Trace("ConnWrapper: ARBITRARY path selection")
		pathSet := resolver.Query(context.Background(), localIA, address.IA, sciond.PathReqFlags{})
		appPath = pathSet.GetAppPath("")
		nextHop, path, err = getSCIONPath(appPath)
		if err != nil {
			return 0, common.NewBasicError(fmt.Sprintf("Conn Wrapper: Arbitrary : error getting SCION path " +
				"between client %v and server %v", localIA.String(), address.IA.String()), err)
		}
		log.Trace(fmt.Sprintf("SELECTED PATH %s\n", appPath.Entry.Path.String()))

	case RoundRobin:
		log.Trace("ConnWrapper: ROUND-ROBIN path selection")
		if len(c.pathKeys) == 0 {
			c.pathMap = resolver.QueryFilter(context.Background(), localIA, address.IA, c.conf.Policy())
			for k, _ := range c.pathMap {
				c.pathKeys = append(c.pathKeys, k)
			}
		}

		//sanity checks
		if c.nextKeyIndex >= len(c.pathKeys) || len(c.pathKeys) != len(c.pathMap) {
			return 0, common.NewBasicError("Writer: inconsistent path keys array/map length", err)
		}

		appPath, ok := c.pathMap[c.pathKeys[c.nextKeyIndex]]
		log.Trace(fmt.Sprintf("SELECTED PATH # %d: %s\n", c.nextKeyIndex, appPath.Entry.Path.String()))
		if !ok {
			return 0, common.NewBasicError("Writer: Path key not found", nil )
		}
		c.nextKeyIndex = (c.nextKeyIndex + 1) % len(c.pathKeys)

		nextHop, path, err = getSCIONPath(appPath)
		if err != nil {
			return 0, common.NewBasicError(fmt.Sprintf("Conn Wrapper: Round-robin : error getting SCION path" +
				" between client %v and server %v", localIA.String(), address.IA.String()), err)
		}

	default:
		return 0, common.NewBasicError("Path selection option not yet supported" , nil)
	}
	remoteAddr.NextHop, remoteAddr.Path = nextHop, path
	return c.WriteToSCION(b, remoteAddr)
}


func getSCIONPath(appPath *spathmeta.AppPath) (*overlay.OverlayAddr, *spath.Path, error) {
	if appPath == nil {
		return nil, nil, common.NewBasicError(ErrNoPath, nil)
	}
	path := &spath.Path{Raw: appPath.Entry.Path.FwdPath}
	if err := path.InitOffsets(); err != nil {
		return nil, nil, common.NewBasicError(ErrInitPath, nil)
	}
	overlayAddr, err := appPath.Entry.HostInfo.Overlay()
	if err != nil {
		return nil, nil, common.NewBasicError(ErrBadOverlay, nil)
	}
	return overlayAddr, path, nil

}





