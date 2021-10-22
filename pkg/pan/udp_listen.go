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
	"os"
	"sync"
	"time"
)

var errBadDstAddress error = errors.New("dst address not a UDPAddr")

// ReplySelector selects the reply path for WriteTo in a listenConn.
type ReplySelector interface {
	ReplyPath(src, dst UDPAddr) *Path
	OnPacketReceived(src, dst UDPAddr, path *Path)
	OnPathDown(PathFingerprint, PathInterface)
	Close() error
}

type ListenConn interface {
	net.PacketConn
	// ReadFromPath reads a message and returns the (return-)path via which the
	// message was received.
	// This bypasses selector's OnPacketReceived used for ReadFrom.
	ReadFromPath(b []byte) (int, UDPAddr, *Path, error)
	// WriteToPath writes a message to the remote address via the given path.
	// This bypasses selector used for WriteTo.
	WriteToPath(b []byte, dst UDPAddr, path *Path) (int, error)
}

func ListenUDP(ctx context.Context, local *net.UDPAddr,
	selector ReplySelector) (ListenConn, error) {

	local, err := defaultLocalAddr(local)
	if err != nil {
		return nil, err
	}

	if selector == nil {
		selector = NewDefaultReplySelector()
	}
	stats.subscribe(selector)
	raw, slocal, err := openBaseUDPConn(ctx, local)
	if err != nil {
		return nil, err
	}

	if len(os.Getenv("SCION_GO_INTEGRATION")) > 0 {
		fmt.Printf("Listening addr=%s\n", slocal)
	}

	return &listenConn{
		baseUDPConn: baseUDPConn{
			raw: raw,
		},
		local:    slocal,
		selector: selector,
	}, nil
}

type listenConn struct {
	baseUDPConn

	local    UDPAddr
	selector ReplySelector
}

func (c *listenConn) LocalAddr() net.Addr {
	return c.local
}

func (c *listenConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, remote, path, err := c.ReadFromPath(b)
	c.selector.OnPacketReceived(remote, c.local, path)
	return n, remote, err
}

func (c *listenConn) ReadFromPath(b []byte) (int, UDPAddr, *Path, error) {
	n, remote, fwPath, err := c.baseUDPConn.readMsg(b)
	if err != nil {
		return n, UDPAddr{}, nil, err
	}
	path, err := reversePathFromForwardingPath(remote.IA, c.local.IA, fwPath)
	return n, remote, path, err
}

func (c *listenConn) WriteTo(b []byte, dst net.Addr) (int, error) {
	sdst, ok := dst.(UDPAddr)
	if !ok {
		return 0, errBadDstAddress
	}
	var path *Path
	if c.local.IA != sdst.IA {
		path = c.selector.ReplyPath(c.local, sdst)
		if path == nil {
			return 0, errNoPathTo(sdst.IA)
		}
	}
	return c.WriteToPath(b, sdst, path)
}

func (c *listenConn) WriteToPath(b []byte, dst UDPAddr, path *Path) (int, error) {
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
	remotes map[udpAddrKey]remoteEntry
}

func NewDefaultReplySelector() *DefaultReplySelector {
	return &DefaultReplySelector{
		remotes: make(map[udpAddrKey]remoteEntry),
	}
}

func (s *DefaultReplySelector) ReplyPath(src, dst UDPAddr) *Path {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	r, ok := s.remotes[makeKey(dst)]
	if !ok || len(r.paths) == 0 {
		return nil
	}
	return r.paths[0]
}

func (s *DefaultReplySelector) OnPacketReceived(src, dst UDPAddr, path *Path) {
	if path == nil {
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	ksrc := makeKey(src)
	r := s.remotes[ksrc]
	r.seen = time.Now()
	r.paths.insert(path, defaultSelectorMaxReplyPaths)
	s.remotes[ksrc] = r
}

func (s *DefaultReplySelector) OnPathDown(PathFingerprint, PathInterface) {
	// TODO failover.
}

func (s *DefaultReplySelector) Close() error {
	return nil
}

type udpAddrKey struct {
	IA   IA
	IP   [16]byte
	Port int
}

func makeKey(a UDPAddr) udpAddrKey {
	k := udpAddrKey{
		IA:   a.IA,
		Port: a.Port,
	}
	copy(k.IP[:], a.IP.To16())
	return k
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
