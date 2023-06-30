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
	"net"
	"sort"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/snet"
)

// Policy is a stateless filter / sorter for paths.
type Policy interface {
	// Filter and prioritize paths
	Filter(paths []*Path) []*Path
}

type PolicyFunc func(paths []*Path) []*Path

func (f PolicyFunc) Filter(paths []*Path) []*Path {
	return f(paths)
}

// PolicyChain applies multiple policies in order.
type PolicyChain []Policy

func (p PolicyChain) Filter(paths []*Path) []*Path {
	for _, p := range p {
		paths = p.Filter(paths)
	}
	return paths
}

// Pinned is a policy that keeps only a preselected set of paths.
// This can be used to implement interactive hard path selection.
type Pinned []PathFingerprint

func (p Pinned) Filter(paths []*Path) []*Path {
	filtered := make([]*Path, 0, len(p))
	for _, s := range p {
		for _, path := range paths {
			if path.Fingerprint == s {
				filtered = append(filtered, path)
				break
			}
		}
	}
	return filtered
}

// Preferred is a policy adapter that keeps all paths but moves the paths
// selected by the child policy to the top.
// This can be used, for example, to implement interactive path preference with
// failover to other paths.
type Preferred struct {
	Preferred Policy
}

func (p Preferred) Filter(paths []*Path) []*Path {
	preferred := p.Preferred.Filter(paths)
	rest := make([]*Path, 0, len(paths)-len(preferred))
	for _, path := range paths {
		add := true
		for _, s := range preferred {
			if path.Fingerprint == s.Fingerprint {
				add = false
				break
			}
		}
		if add {
			rest = append(rest, path)
		}
	}
	return append(preferred, rest...)
}

// Sequence is a policy filtering paths matching a textual pattern. The sequence pattern is
// space separated sequence of hop predicates.
// See https://scion.docs.anapaya.net/en/latest/PathPolicy.html#sequence.
type Sequence struct {
	sequence *pathpol.Sequence
}

// NewSequence creates a new sequence from a string
func NewSequence(s string) (Sequence, error) {
	sequence, err := pathpol.NewSequence(s)
	return Sequence{sequence: sequence}, err
}

// Filter evaluates the interface sequence list and returns the set of paths
// that match the list
func (s Sequence) Filter(paths []*Path) []*Path {
	wps := make([]snet.Path, len(paths))
	for i := range paths {
		wps[i] = snetPathWrapper{wrapped: paths[i]}
	}
	wps = s.sequence.Eval(wps)
	ps := make([]*Path, len(wps))
	for i := range wps {
		ps[i] = wps[i].(snetPathWrapper).wrapped
	}
	return ps
}

func (s Sequence) String() string {
	return s.sequence.String()
}

// ACL is a policy filtering paths matching an ACL pattern. The ACL pattern is
// an ordered list of allow/deny actions over hop predicates.
// See https://scion.docs.anapaya.net/en/latest/PathPolicy.html#acl.
type ACL struct {
	entries *pathpol.ACL
}

// NewACL creates a new ACL from a string list
func NewACL(list []string) (ACL, error) {
	aclEntries := make([]*pathpol.ACLEntry, len(list))
	for i, entry := range list {
		aclEntry := &pathpol.ACLEntry{}
		if err := aclEntry.LoadFromString(entry); err != nil {
			return ACL{}, fmt.Errorf("parsing ACL entries: %w", err)
		}
		aclEntries[i] = aclEntry
	}
	acl, err := pathpol.NewACL(aclEntries...)
	if err != nil {
		return ACL{}, fmt.Errorf("creating ACL: %w", err)
	}
	return ACL{entries: acl}, nil
}

func (acl *ACL) UnmarshalJSON(input []byte) error {
	acl.entries = &pathpol.ACL{}
	return acl.entries.UnmarshalJSON(input)
}

func (acl *ACL) String() string {
	output := ""
	for _, entry := range acl.entries.Entries {
		output += entry.String() + ", "
	}
	return output
}

// Filter evaluates the interface ACL and returns the set of paths
// that match the list
func (acl *ACL) Filter(paths []*Path) []*Path {
	wps := make([]snet.Path, len(paths))
	for i := range paths {
		wps[i] = snetPathWrapper{wrapped: paths[i]}
	}
	wps = acl.entries.Eval(wps)
	ps := make([]*Path, len(wps))
	for i := range wps {
		ps[i] = wps[i].(snetPathWrapper).wrapped
	}
	return ps
}

// snetPathWrapper wraps a *Path to snet.Path, only supporting the minimal
// interface to use pathpol.Sequence
type snetPathWrapper struct {
	wrapped *Path
}

func (p snetPathWrapper) UnderlayNextHop() *net.UDPAddr { panic("not implemented") }

func (p snetPathWrapper) Dataplane() snet.DataplanePath { panic("not implemented") }
func (p snetPathWrapper) Destination() addr.IA          { panic("not implemented") }
func (p snetPathWrapper) Copy() snet.Path               { panic("not implemented") }

func (p snetPathWrapper) Metadata() *snet.PathMetadata {
	if p.wrapped.Metadata == nil {
		return nil
	}
	pis := make([]snet.PathInterface, len(p.wrapped.Metadata.Interfaces))
	for i, spi := range p.wrapped.Metadata.Interfaces {
		pis[i] = snet.PathInterface{
			IA: addr.IA(spi.IA),
			ID: common.IFIDType(spi.IfID), //nolint:staticcheck // False deprecation
		}
	}
	return &snet.PathMetadata{
		Interfaces: pis,
	}
}

// TODO: (optionally) fill missing latency info with geo coordinates
type LowestLatency struct{}

func (p LowestLatency) Filter(paths []*Path) []*Path {
	sortStablePartialOrder(paths, func(i, j int) (bool, bool) {
		return paths[i].Metadata.LowerLatency(paths[j].Metadata)
	})
	return paths
}

type HighestBandwidth struct{}

func (p HighestBandwidth) Filter(paths []*Path) []*Path {
	sortStablePartialOrder(paths, func(i, j int) (bool, bool) {
		return paths[i].Metadata.HigherBandwidth(paths[j].Metadata)
	})
	return paths
}

type LeastHops struct{}

func (p LeastHops) Filter(paths []*Path) []*Path {
	sort.SliceStable(paths, func(i, j int) bool {
		return len(paths[i].Metadata.Interfaces) < len(paths[j].Metadata.Interfaces)
	})
	return paths
}

type HighestMTU struct{}

func (p HighestMTU) Filter(paths []*Path) []*Path {
	sort.SliceStable(paths, func(i, j int) bool {
		return paths[i].Metadata.MTU > paths[j].Metadata.MTU
	})
	return paths
}

// sortStablePartialOrder sorts the path slice according to the given function
// defining a partial order.
// The less function is expected to return:
//   true,  true if s[i] < s[j]
//   false, true if s[i] >= s[j]
//   _    , false otherwise, i.e. if s[i] and s[j] are not comparable
//
// NOTE: this is implemented as an insertion sort, so has quadratic complexity.
// Should not be called with more than very few hundred paths. Be careful!
func sortStablePartialOrder(s []*Path, lessFunc func(i, j int) (bool, bool)) {
	for i := 1; i < len(s); i++ {
		k := i
		for j := k - 1; j >= 0; j-- {
			less, ok := lessFunc(k, j)
			if ok && less {
				s[j], s[k] = s[k], s[j]
				k = j
			} else if ok && !less {
				// elements before i already in order. If s[k] >= s[j], then this is also
				// true for all comparable elements before j.
				break
			}
		}
	}
}
