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

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/snet"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

// error strings
const (
	errNoPath = "path not found"
)

// PathSelector selects a path for a given address.
type PathSelector interface {
	// Reset initializes this path selector
	Reset([]snet.Path) error
	// Get implements the path selection logic specified in PathAppConf
	Next() snet.Path
}

// staticPathSelector implements static path selection
// The connection uses the same path used in the first call to WriteTo for all
// subsequenet packets
type staticPathSelector struct {
	staticPath snet.Path
}

func (s *staticPathSelector) Reset(paths []snet.Path) error {
	s.staticPath = paths[0]
	return nil
}

func (s *staticPathSelector) Next() snet.Path {
	return s.staticPath
}

// roundrobinPathSelector implements round-robin path selection For N
// arbitrarily ordered paths, the ith call for WriteTo uses the (i % N)th path
type roundRobinPathSelector struct {
	paths        []snet.Path
	nextKeyIndex int
}

func (s *roundRobinPathSelector) Reset(paths []snet.Path) error {
	s.paths = paths
	s.nextKeyIndex = s.nextKeyIndex % len(paths)
	return nil
}

func (s *roundRobinPathSelector) Next() snet.Path {
	path := s.paths[s.nextKeyIndex]
	s.nextKeyIndex = (s.nextKeyIndex + 1) % len(s.paths)
	return path
}

// policyConn is a wrapper class around snet.SCIONConn that overrides its WriteTo function,
// so that it chooses the path on which the packet is written.
type policyConn struct {
	net.PacketConn
	conf      *PathAppConf
	mutex     sync.Mutex
	selectors map[addr.IA]PathSelector
}

// NewPolicyConn constructs a PolicyConn specified in the PathAppConf argument.
func NewPolicyConn(c snet.Conn, conf *PathAppConf) net.PacketConn {

	return &policyConn{
		PacketConn: c,
		conf:       conf,
		selectors:  make(map[addr.IA]PathSelector),
	}
}

// WriteTo wraps snet.SCIONConn.WriteTo
func (c *policyConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	address, ok := raddr.(*snet.UDPAddr)
	if !ok {
		return 0, errors.New("unable to write to non-SCION address")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	selector, err := c.getSelector(address.IA)
	if err != nil {
		return 0, err
	}
	var path snet.Path
	if selector != nil { // nil for local IA
		path = selector.Next()
	}
	appnet.SetPath(address, path)
	return c.PacketConn.WriteTo(b, address)
}

func (c *policyConn) getSelector(ia addr.IA) (PathSelector, error) {

	if selector, ok := c.selectors[ia]; ok {
		return selector, nil
	}
	if ia == appnet.DefNetwork().IA {
		return nil, nil
	}
	selector, err := c.constructSelector(ia)
	if err != nil {
		return nil, err
	}
	c.selectors[ia] = selector
	return selector, nil
}

func (c *policyConn) constructSelector(ia addr.IA) (PathSelector, error) {

	selector := newSelector(c.conf.PathSelection())
	paths, err := queryPathsFiltered(ia, c.conf.Policy())
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New(errNoPath)
	}
	err = selector.Reset(paths)
	if err != nil {
		return nil, err
	}
	return selector, nil
}

func newSelector(selection PathSelection) PathSelector {
	switch selection {
	case RoundRobin:
		return &roundRobinPathSelector{}
	default:
		// Static or Arbitrary
		// XXX(matzf): remove Arbitrary and make Static the default?
		return &staticPathSelector{}
	}
}

func queryPathsFiltered(ia addr.IA, policy *pathpol.Policy) ([]snet.Path, error) {
	paths, err := appnet.QueryPaths(ia)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return paths, nil
	}

	pathSet := make(pathpol.PathSet)
	for _, path := range paths {
		pathSet[path.Fingerprint()] = path
	}
	policy.Filter(pathSet)
	filterPathSlice(&paths, pathSet)
	return paths, nil
}

// filterPathSlice keeps only paths that are in pathSet, leaving the order of the slice intact
func filterPathSlice(paths *[]snet.Path, pathSet pathpol.PathSet) {

	// Nasty "idiomatic" slice filtering: https://stackoverflow.com/a/50183212
	filtered := (*paths)[:0]
	for _, p := range *paths {
		if _, ok := pathSet[p.Fingerprint()]; ok {
			filtered = append(filtered, p)
		}
	}
	*paths = filtered
}
