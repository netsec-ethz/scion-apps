// Copyright 2021 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/scionproto/scion/pkg/slayers"
	"github.com/scionproto/scion/pkg/slayers/extension"
	"github.com/scionproto/scion/pkg/slayers/path/scion"
	"github.com/scionproto/scion/pkg/snet"
)

var errBadDstAddress error = errors.New("dst address not a UDPAddr")

// ReplySelector controls the reply path in a **listening** socket. Stateful.
type ReplySelector interface {
	// Path selects the path for the next packet to remote.
	// Invoked for each packet sent with WriteTo.
	Path(remote UDPAddr) *Path
	// Initialize the selector.
	// Invoked once during the creation of a ListenConn.
	Initialize(local UDPAddr)
	// Record a path used by the remote for a packet received.
	// Invoked whenever a packet is received.
	// The path is reversed, i.e. it's the path from here to remote.
	Record(remote UDPAddr, path *Path)
	// PathDown is called whenever an SCMP down notification is received on any
	// connection so that the selector can adapt its path choice. The down
	// notification may be for unrelated paths not used by this selector.
	PathDown(PathFingerprint, PathInterface)
	Close() error
}

type ListenConn interface {
	net.PacketConn
	// ReadFromVia reads a message and returns the (return-)path via which the
	// message was received.
	ReadFromVia(b []byte) (int, UDPAddr, *Path, error)
	// WriteToVia writes a message to the remote address via the given path.
	// This bypasses selector used for WriteTo.
	WriteToVia(b []byte, dst UDPAddr, path *Path) (int, error)
}

func ListenUDP(ctx context.Context, local netip.AddrPort,
	selector ReplySelector) (ListenConn, error) {

	local, err := defaultLocalAddr(local)
	if err != nil {
		return nil, err
	}

	if selector == nil {
		selector = NewDefaultReplySelector()
	}
	stats.subscribe(selector)
	sn := snet.SCIONNetwork{
		Topology:    host().sciond,
		SCMPHandler: scmpHandler{},
	}
	conn, err := sn.OpenRaw(ctx, net.UDPAddrFromAddrPort(local))
	if err != nil {
		return nil, err
	}
	ipport := conn.LocalAddr().(*net.UDPAddr).AddrPort()
	localUDPAddr := UDPAddr{
		IA:   host().ia,
		IP:   ipport.Addr(),
		Port: ipport.Port(),
	}
	selector.Initialize(localUDPAddr)

	if len(os.Getenv("SCION_GO_INTEGRATION")) > 0 {
		fmt.Printf("Listening addr=%s\n", localUDPAddr)
	}

	return &listenConn{
		baseUDPConn: baseUDPConn{
			raw: conn,
		},
		local:    localUDPAddr,
		selector: selector,
	}, nil
}

func ListenUDPWithFabrid(ctx context.Context, local netip.AddrPort, remote UDPAddr,
	selector ReplySelector) (ListenConn, error) {

	local, err := defaultLocalAddr(local)
	if err != nil {
		return nil, err
	}

	if selector == nil {
		selector = NewDefaultReplySelector()
	}
	stats.subscribe(selector)
	sn := snet.SCIONNetwork{
		Topology:    host().sciond,
		SCMPHandler: scmpHandler{},
	}
	conn, err := sn.OpenRaw(ctx, net.UDPAddrFromAddrPort(local))
	if err != nil {
		return nil, err
	}
	ipport := conn.LocalAddr().(*net.UDPAddr).AddrPort()
	localUDPAddr := UDPAddr{
		IA:   host().ia,
		IP:   ipport.Addr(),
		Port: ipport.Port(),
	}
	selector.Initialize(localUDPAddr)

	if len(os.Getenv("SCION_GO_INTEGRATION")) > 0 {
		fmt.Printf("Listening addr=%s\n", localUDPAddr)
	}

	server := NewFabridServer(ctx, localUDPAddr, remote)
	return &fabridListenConn{
		listenConn: listenConn{
			baseUDPConn: baseUDPConn{
				raw: conn,
			},
			local:    localUDPAddr,
			selector: selector,
		},
		fabridServer: server,
	}, nil
}

type listenConn struct {
	baseUDPConn

	local    UDPAddr
	selector ReplySelector
}

type fabridListenConn struct {
	listenConn

	fabridServer *FabridServer
}

func (c *listenConn) LocalAddr() net.Addr {
	return c.local
}

func (c *listenConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, remote, _, err := c.ReadFromVia(b)
	return n, remote, err
}

func (c *listenConn) ReadFromVia(b []byte) (int, UDPAddr, *Path, error) {
	n, remote, fwPath, _, _, err := c.baseUDPConn.readMsg(b)
	if err != nil {
		return n, UDPAddr{}, nil, err
	}
	path, err := reversePathFromForwardingPath(remote.IA, c.local.IA, fwPath)
	c.selector.Record(remote, path)
	return n, remote, path, err
}

func (c *listenConn) WriteTo(b []byte, dst net.Addr) (int, error) {
	sdst, ok := dst.(UDPAddr)
	if !ok {
		return 0, errBadDstAddress
	}
	var path *Path
	if c.local.IA != sdst.IA {
		path = c.selector.Path(sdst)
		if path == nil {
			return 0, errNoPathTo(sdst.IA)
		}
	}
	return c.WriteToVia(b, sdst, path)
}

func (c *listenConn) WriteToVia(b []byte, dst UDPAddr, path *Path) (int, error) {
	return c.baseUDPConn.writeMsg(c.local, dst, path, b)
}

func (c *listenConn) Close() error {
	stats.unsubscribe(c.selector)
	// FIXME: multierror!
	_ = c.selector.Close()
	return c.baseUDPConn.Close()
}

type DefaultReplySelector struct {
	mtx     sync.RWMutex
	remotes map[UDPAddr]remoteEntry
}

func NewDefaultReplySelector() *DefaultReplySelector {
	return &DefaultReplySelector{
		remotes: make(map[UDPAddr]remoteEntry),
	}
}

func (c *fabridListenConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, remote, _, err := c.ReadFromVia(b)
	return n, remote, err
}

func (c *fabridListenConn) ReadFromVia(b []byte) (int, UDPAddr, *Path, error) {
	n, panRemote, fwPath, hbhExt, _, err := c.baseUDPConn.readMsg(b)
	if err != nil {
		return n, UDPAddr{}, nil, err
	}

	path, err := reversePathFromForwardingPath(panRemote.IA, c.local.IA, fwPath)
	c.selector.Record(panRemote, path)

	// Check extensions for relevant options
	var identifierOption *extension.IdentifierOption
	var fabridOption *extension.FabridOption
	if hbhExt != nil {
		for _, opt := range hbhExt.Options {
			switch opt.OptType {
			case slayers.OptTypeIdentifier:
				decoded := scion.Decoded{}
				err = decoded.DecodeFromBytes(fwPath.dataplanePath.(snet.RawPath).Raw)
				if err != nil {
					return 0, UDPAddr{}, nil, err
				}
				baseTimestamp := decoded.InfoFields[0].Timestamp
				identifierOption, err = extension.ParseIdentifierOption(opt, baseTimestamp)
				if err != nil {
					return 0, UDPAddr{}, nil, err
				}
			case slayers.OptTypeFabrid:
				fabridOption, err = extension.ParseFabridOptionFullExtension(opt, (opt.OptDataLen-4)/4)
				if err != nil {
					return 0, UDPAddr{}, nil, err
				}
			}
		}
	}
	if fabridOption != nil && identifierOption != nil {

		err = c.fabridServer.HandleFabridPacket(fabridOption, identifierOption)
		if err != nil {
			return 0, UDPAddr{}, nil, err
		}
	}

	return n, panRemote, path, err
}

func (s *DefaultReplySelector) Initialize(local UDPAddr) {
}

func (s *DefaultReplySelector) Path(remote UDPAddr) *Path {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	r, ok := s.remotes[remote]
	if !ok || len(r.paths) == 0 {
		return nil
	}
	return r.paths[0]
}

func (s *DefaultReplySelector) Record(remote UDPAddr, path *Path) {
	if path == nil {
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	r := s.remotes[remote]
	r.seen = time.Now()
	r.paths.insert(path, defaultSelectorMaxReplyPaths)
	s.remotes[remote] = r
}

func (s *DefaultReplySelector) PathDown(PathFingerprint, PathInterface) {
	// TODO failover.
}

func (s *DefaultReplySelector) Close() error {
	return nil
}

type remoteEntry struct {
	paths pathsMRU
	seen  time.Time
}

// pathsMRU is a list tracking the most recently used (inserted) path
type pathsMRU []*Path

func (p *pathsMRU) insert(path *Path, maxEntries int) {
	paths := *p
	i := 0
	for ; i < len(paths); i++ {
		if paths[i].Fingerprint == path.Fingerprint {
			break
		}
	}
	if i == len(paths) {
		if len(paths) < maxEntries {
			*p = append(paths, nil)
			paths = *p
		} else {
			i = len(paths) - 1 // overwrite least recently used
		}
	}
	paths[i] = path

	// move most-recently-used to front
	if i != 0 {
		pi := paths[i]
		copy(paths[1:i+1], paths[0:i])
		paths[0] = pi
	}
}
