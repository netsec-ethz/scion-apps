// Copyright 2020 ETH Zurich
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

package scionutils

import (
	"errors"
	"net"
	"sync"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"

	"github.com/scionproto/scion/go/lib/snet"
)

// error strings
const (
	ErrNoPath = "path not found"
)

// PathSelector selects a path for a given address.
type PathSelector interface {
	// SelectPath implements the path selection logic specified in PathAppConf
	SelectPath(address *snet.Addr) (snet.Path, error)
}

type defaultPathSelector struct{}

// Default behavior is arbitrary path selection
func (s *defaultPathSelector) SelectPath(address *snet.Addr) (snet.Path, error) {
	paths, err := appnet.QueryPaths(address.IA)
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	return paths[0], nil
}

// staticPathSelector implements static path selection
// The connection uses the same path used in the first call to WriteTo for all
// subsequenet packets
type staticPathSelector struct {
	staticPath snet.Path
	initOnce   sync.Once
}

func (s *staticPathSelector) SelectPath(address *snet.Addr) (snet.Path, error) {
	// TODO(matzf): separate initialization to simplify proper error handling
	// TODO(matzf): query filter (using pathpol.Policy.Filter)
	s.initOnce.Do(func() {
		paths, _ := appnet.QueryPaths(address.IA)
		if len(paths) > 0 {
			s.staticPath = paths[0]
		}
	})

	return s.staticPath, nil
}

// roundrobinPathSelector implements round-robin path selection For N
// arbitrarily ordered paths, the ith call for WriteTo uses the (i % N)th path
type roundRobinPathSelector struct {
	paths        []snet.Path
	nextKeyIndex int
	initOnce     sync.Once
}

func (s *roundRobinPathSelector) SelectPath(address *snet.Addr) (snet.Path, error) {
	s.initOnce.Do(func() {
		s.paths, _ = appnet.QueryPaths(address.IA)
	})

	path := s.paths[s.nextKeyIndex]
	s.nextKeyIndex = (s.nextKeyIndex + 1) % len(s.paths)
	return path, nil
}

// policyConn is a wrapper class around snet.SCIONConn that overrides its WriteTo function,
// so that it chooses the path on which the packet is written.
type policyConn struct {
	net.PacketConn
	pathSelector PathSelector
}

var _ net.PacketConn = (*policyConn)(nil)

// WriteTo wraps snet.SCIONConn.WriteTo
func (c *policyConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, errors.New("unable to write to non-SCION address")
	}
	path, err := c.pathSelector.SelectPath(address)
	if err != nil {
		return 0, err
	}
	appnet.SetPath(address, path)
	return c.PacketConn.WriteTo(b, address)
}

// NewPolicyConn constructs a PolicyConn specified in the PathAppConf argument.
func NewPolicyConn(c snet.Conn, conf *PathAppConf) net.PacketConn {

	var pathSel PathSelector
	switch conf.PathSelection() {
	case Static:
		pathSel = &staticPathSelector{}
	case RoundRobin:
		pathSel = &roundRobinPathSelector{}
	default:
		pathSel = &defaultPathSelector{}
	}
	return &policyConn{
		PacketConn:   c,
		pathSelector: pathSel,
	}
}
