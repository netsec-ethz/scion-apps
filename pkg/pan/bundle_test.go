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
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSelectorBundleSelectorsList checks that the management of the selectors
// list in the bundle works correctly.
func TestSelectorBundleSelectorsList(t *testing.T) {
	bundledSelectors := func(selectors ...Selector) []*bundledSelector {
		t.Helper()
		r := make([]*bundledSelector, len(selectors))
		for i, s := range selectors {
			r[i] = s.(*bundledSelector)
		}
		return r
	}

	b := &SelectorBundle{}
	s1 := b.New()
	assert.Equal(t, bundledSelectors(s1), b.selectors)
	s2 := b.New()
	assert.Equal(t, bundledSelectors(s1, s2), b.selectors)
	s2.Close()
	assert.Equal(t, bundledSelectors(s1), b.selectors)
	s3 := b.New()
	assert.Equal(t, bundledSelectors(s1, s3), b.selectors)
	s1.Close()
	assert.Equal(t, bundledSelectors(s3), b.selectors)
	s3.Close()
	assert.Equal(t, bundledSelectors(), b.selectors)
	s4 := b.New()
	assert.Equal(t, bundledSelectors(s4), b.selectors)
}

func TestBundlePathUsageOverlap(t *testing.T) {
	mustParseIntf := func(intfShort string) PathInterface {
		parts := strings.Split(intfShort, "#")
		if len(parts) != 2 {
			panic(fmt.Sprintf("bad interface %q", intfShort))
		}
		ia := MustParseIA("1-ff00:0:" + parts[0])
		ifid, _ := strconv.Atoi(parts[1])
		return PathInterface{IA: ia, IfID: IfID(ifid)}
	}

	makePath := func(intfStrs ...string) *Path {
		intfs := make([]PathInterface, len(intfStrs))
		for i, intfStr := range intfStrs {
			intfs[i] = mustParseIntf(intfStr)
		}
		return &Path{
			Metadata: &PathMetadata{
				Interfaces: intfs,
			},
		}
	}

	pAB1 := makePath("a#1", "b#1")
	pAB2 := makePath("a#2", "b#2")
	pACB11 := makePath("a#11", "c#11", "c#12", "b#11")
	pACB22 := makePath("a#12", "c#13", "c#14", "b#12")
	pACB12 := makePath("a#11", "c#11", "c#14", "b#12")
	pACB21 := makePath("a#12", "c#13", "c#12", "b#11")
	pADB := makePath("a#r21", "d#21", "d#22", "b#21")
	pAD := makePath("a#r21", "d#21")

	overlapCases := []struct {
		name    string
		usage   []*Path
		path    *Path
		overlap int
	}{
		{
			name:    "empty",
			usage:   nil,
			path:    pAB1,
			overlap: 0,
		},
		{
			name:    "disjoint",
			usage:   []*Path{pAB1},
			path:    pAB2,
			overlap: 0,
		},
		{
			name:    "same",
			usage:   []*Path{pAB1},
			path:    pAB1,
			overlap: 1,
		},
		{
			name:    "same twice",
			usage:   []*Path{pAB1, pAB1},
			path:    pAB1,
			overlap: 2,
		},
		{
			name:    "via D 1",
			usage:   []*Path{pAD},
			path:    pADB,
			overlap: 1,
		},
		{
			name:    "via D 2",
			usage:   []*Path{pADB},
			path:    pAD,
			overlap: 1,
		},
		{
			name:    "crisscross",
			usage:   []*Path{pACB11, pACB12, pACB21, pACB22},
			path:    pACB12,
			overlap: 2,
		},
	}
	for _, c := range overlapCases {
		t.Run("overlap "+c.name, func(t *testing.T) {
			u := newBundlePathUsage()
			for _, p := range c.usage {
				u.add(p)
			}
			actual := u.overlap(c.path)
			assert.Equal(t, c.overlap, actual)
		})
	}

	maxDisjointCases := []struct {
		name     string
		usage    []*Path
		paths    []*Path
		expected int
	}{
		{
			name:     "empty",
			usage:    nil,
			paths:    []*Path{pAB1, pAB2},
			expected: 0,
		},
		{
			name:     "avoid existing",
			usage:    []*Path{pAB1},
			paths:    []*Path{pAB1, pAB2},
			expected: 1,
		},
		{
			name:     "balance 1",
			usage:    []*Path{pAB1, pAB1, pAB2},
			paths:    []*Path{pAB1, pAB2},
			expected: 1,
		},
		{
			name:     "balance 2",
			usage:    []*Path{pAB1, pAB2, pAB2},
			paths:    []*Path{pAB1, pAB2},
			expected: 0,
		},
	}
	for _, c := range maxDisjointCases {
		t.Run("firstMaxDisjoint "+c.name, func(t *testing.T) {
			u := newBundlePathUsage()
			for _, p := range c.usage {
				u.add(p)
			}
			actual := u.firstMaxDisjoint(c.paths)
			assert.Equal(t, c.expected, actual)
		})
	}
}
