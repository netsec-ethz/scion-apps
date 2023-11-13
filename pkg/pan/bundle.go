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
	"fmt"
	"sync"
)

// SelectorBundle combines the selectors for multiple connections, attempting
// to use (and keep using, e.g under changing policies and paths expring or
// going down) disjoint paths.
// The idea is that the bandwidths of different paths can be accumulated and
// the load of the indivdual connections can be distributed over different
// network links.
//
// First create a SelectorBundle object and then use New() to create selectors
// for each individual Conn (i.e. for each Dial call).
// The bundle can be used for connections to different destinations.
//
// This first implementation of this bundle concept is somewhat primitive and
// only determines path usage by the number of connections using a path.
// Later on, we may want to consider bandwidth information from the path
// metadata, allow customized selectors like e.g. the pinging selector, allow
// defining priorities, etc.
type SelectorBundle struct {
	mutex     sync.Mutex
	selectors []*bundledSelector
}

// New creates a Selector for a dialed connection. The path chosen by this selector
// will attempt to minimize the usage overlap with other selectors from this bundle.
func (b *SelectorBundle) New() Selector {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	s := &bundledSelector{
		bundle: b,
	}
	b.selectors = append(b.selectors, s)
	return s
}

func (b *SelectorBundle) remove(s *bundledSelector) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if idx, ok := b.index(s); ok {
		b.selectors = append(b.selectors[:idx], b.selectors[idx+1:]...)
	}
}

func (b *SelectorBundle) index(s *bundledSelector) (int, bool) {
	for i := range b.selectors {
		if b.selectors[i] == s {
			return i, true
		}
	}
	return -1, false
}

func (b *SelectorBundle) firstMaxDisjoint(self *bundledSelector, paths []*Path) int {
	if len(paths) <= 1 {
		return 0
	}

	// build up path usage information
	u := newBundlePathUsage()
	for _, s := range b.selectors {
		if s == self {
			continue
		}
		if p := s.Path(); p != nil {
			u.add(p)
		}
	}

	return u.firstMaxDisjoint(paths)
}

func (b *SelectorBundle) firstMaxDisjointMoreAlive(self *bundledSelector, current *Path, paths []*Path) int {
	moreAlive := make([]*Path, 0, len(paths))
	for _, p := range paths {
		if stats.IsMoreAlive(p, current) {
			moreAlive = append(moreAlive, p)
		}
	}
	if len(moreAlive) == 0 {
		return b.firstMaxDisjoint(self, paths)
	}
	best := moreAlive[b.firstMaxDisjoint(self, moreAlive)]
	for i, p := range paths {
		if p == best {
			return i
		}
	}
	panic("logic error, expected to find selected max disjoint in paths slice")
}

// bundlePathUsage tracks the path usage by the selectors of a bundle.
// Currently, only per-interface usage counts are tracked.
type bundlePathUsage struct {
	intfs map[PathInterface]int
}

func newBundlePathUsage() bundlePathUsage {
	return bundlePathUsage{
		intfs: make(map[PathInterface]int),
	}
}

func (u *bundlePathUsage) add(p *Path) {
	intfs := p.Metadata.Interfaces
	for _, intf := range intfs {
		u.intfs[intf] = u.intfs[intf] + 1
	}
}

// overlap returns how many (other) connections/selectors use paths that
// overlap with p (i.e. use the same path interfaces / hops).
func (u *bundlePathUsage) overlap(p *Path) (overlap int) {
	intfs := p.Metadata.Interfaces
	for _, intf := range intfs {
		overlap = maxInt(overlap, u.intfs[intf])
	}
	return
}

func (u *bundlePathUsage) firstMaxDisjoint(paths []*Path) int {
	best := 0
	bestOverlap := u.overlap(paths[0])
	for i := 1; i < len(paths); i++ {
		overlap := u.overlap(paths[i])
		if overlap < bestOverlap {
			best, bestOverlap = i, overlap
		}
	}
	return best
}

// bundledSelector is a Selector in a SelectorBundle.
type bundledSelector struct {
	bundle  *SelectorBundle
	mutex   sync.Mutex
	paths   []*Path
	current int
}

func (s *bundledSelector) Path() *Path {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.paths) == 0 {
		return nil
	}
	return s.paths[s.current]
}

func (s *bundledSelector) Initialize(local, remote UDPAddr, paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.paths = paths
	s.current = s.bundle.firstMaxDisjoint(s, paths)
}

func (s *bundledSelector) Refresh(paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newcurrent := -1
	if len(s.paths) > 0 {
		currentFingerprint := s.paths[s.current].Fingerprint
		for i, p := range paths {
			if p.Fingerprint == currentFingerprint {
				newcurrent = i
				break
			}
		}
	}
	if newcurrent < 0 {
		newcurrent = s.bundle.firstMaxDisjoint(s, paths)
	}
	s.paths = paths
	s.current = newcurrent
}

func (s *bundledSelector) PathDown(pf PathFingerprint, pi PathInterface) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	current := s.paths[s.current]
	if isInterfaceOnPath(current, pi) || pf == current.Fingerprint {
		fmt.Println("down:", s.current, len(s.paths))
		better := s.bundle.firstMaxDisjointMoreAlive(s, current, s.paths)
		if better >= 0 {
			s.current = better
			fmt.Println("failover:", s.current, len(s.paths))
		}
	}
}

func (s *bundledSelector) Close() error {
	s.bundle.remove(s)
	return nil
}
