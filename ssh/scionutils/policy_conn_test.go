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
	"net"
	"reflect"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

//All tests in this file test the correctness of the path selection modes (round-robin, static)
//The assumption is that path filtering has already been tested in SCIONProto

func TestPolicyConn_SelectorType(t *testing.T) {
	tables := []struct {
		pathSelection PathSelection
		policyConn    PathSelector
	}{
		{Arbitrary, &staticPathSelector{}},
		{RoundRobin, &roundRobinPathSelector{}},
		{Static, &staticPathSelector{}},
	}

	for _, table := range tables {
		selector := newSelector(table.pathSelection)

		resultType := reflect.TypeOf(selector)
		expectedType := reflect.TypeOf(table.policyConn)
		if resultType != expectedType {
			t.Errorf("PolicyConnFromConfig expecting path selector type %s, got type %s", expectedType, resultType)
		}
	}
}

func TestPolicyConn_StaticPathSelector(t *testing.T) {

	const numPaths = 5
	const numRepetitions = 3
	paths := makePaths(numPaths)

	selector := newSelector(Static)
	selector.Reset(paths)

	for i := 0; i < numRepetitions*numPaths; i++ {
		expected := paths[0]
		actual := selector.Next()
		if actual != expected {
			t.Fatalf("Static path selection: Expected path %v, found path %v", expected, actual)
		}
	}
}

func TestPolicyConn_RoundRobinSelector(t *testing.T) {

	const numPaths = 5
	const numRepetitions = 3
	paths := makePaths(numPaths)

	roundRobinSeq := []snet.Path{}
	for i := 0; i < numRepetitions; i++ {
		roundRobinSeq = append(roundRobinSeq, paths...)
	}

	selector := newSelector(RoundRobin)
	selector.Reset(paths)

	for i := 0; i < numRepetitions*numPaths; i++ {
		expected := roundRobinSeq[i]
		actual := selector.Next()
		if actual != expected {
			t.Fatalf("Round robin path selection: Expected path %v, found path %v", expected, actual)
		}
	}
}

// mockPath satisfies the snet.Path interface but does not actually implement anything.
type mockPath struct{}

func (p *mockPath) UnderlayNextHop() *net.UDPAddr { return nil }
func (p *mockPath) Path() spath.Path              { return spath.Path{} }
func (p *mockPath) Destination() addr.IA          { return addr.IA{} }
func (p *mockPath) Metadata() *snet.PathMetadata  { return nil }
func (p *mockPath) Copy() snet.Path               { return nil }

func makePaths(num int) []snet.Path {
	paths := make([]snet.Path, num)
	for i := 0; i < num; i++ {
		paths[i] = &mockPath{}
	}
	return paths
}
