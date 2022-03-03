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
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortStablePartialOrder(t *testing.T) {
	// containsAll tests if all runes of sub are contained in s.
	containsAll := func(s, sub string) bool {
		for _, r := range sub {
			if !strings.ContainsRune(s, r) {
				return false
			}
		}
		return true
	}
	// subset is *the* classical partial order.
	// We represent sets using (the rune-set of) strings for brief notation.
	//
	// returns true, true if a < b (a is a strict subset of b)
	//         false, true if a >= b (a is a superset of b)
	//         _, false if a and b are not comparable
	subset := func(a, b string) (bool, bool) {
		if containsAll(a, b) {
			return false, true
		} else if containsAll(b, a) {
			return true, true
		}
		return false, false
	}

	// test the test: checks for the subset function:
	sanityChecks := []struct {
		name     string
		a, b     string
		less, ok bool
	}{
		{
			name: "subset sanity check, equal",
			a:    "a",
			b:    "a",
			less: false,
			ok:   true,
		},
		{
			name: "subset sanity check, subset",
			a:    "a",
			b:    "ab",
			less: true,
			ok:   true,
		},
		{
			name: "subset sanity check, superset",
			a:    "ab",
			b:    "a",
			less: false,
			ok:   true,
		},
		{
			name: "subset sanity check, neither",
			a:    "a",
			b:    "b",
			less: false,
			ok:   false,
		},
	}
	for _, sc := range sanityChecks {
		t.Run(sc.name, func(t *testing.T) {
			less, ok := subset(sc.a, sc.b)
			assert.Equal(t, sc.less, less)
			assert.Equal(t, sc.ok, ok)
		})
	}

	// n**2 --> 400 is still fairly quick, 600 is meh, 1000 takes long, 10000 takes hours
	stressSorted := make([]PathFingerprint, 400)
	sb := strings.Builder{}
	for i := range stressSorted {
		stressSorted[i] = PathFingerprint(sb.String())
		sb.WriteRune(rune(i))
	}
	stressInput := append([]PathFingerprint{}, stressSorted...)
	rand.Shuffle(len(stressInput), func(i, j int) {
		stressInput[i], stressInput[j] = stressInput[j], stressInput[i]
	})

	// test the sorting
	cases := []struct {
		name string
		in   []PathFingerprint
		out  []PathFingerprint
	}{
		{
			name: "nil",
			in:   nil,
			out:  nil,
		},
		{
			name: "empty",
			in:   []PathFingerprint{},
			out:  []PathFingerprint{},
		},
		{
			name: "only incomparable",
			in:   []PathFingerprint{"a", "b"},
			out:  []PathFingerprint{"a", "b"},
		},
		{
			name: "only comparable",
			in:   []PathFingerprint{"ab", "abc", "a"},
			out:  []PathFingerprint{"a", "ab", "abc"},
		},
		// mixed 1-5 inputs contain the same sets but in different order.
		// The sorting is supposed to be *stable*.
		{
			name: "mixed 1",
			in:   []PathFingerprint{"ae", "ab", "abc", "abd", "a"},
			out:  []PathFingerprint{"a", "ae", "ab", "abc", "abd"},
		},
		{
			name: "mixed 2",
			in:   []PathFingerprint{"ab", "ae", "abc", "abd", "a"},
			out:  []PathFingerprint{"a", "ab", "ae", "abc", "abd"},
		},
		{
			name: "mixed 3",
			in:   []PathFingerprint{"ab", "ae", "abd", "a", "abc"},
			out:  []PathFingerprint{"a", "ab", "ae", "abd", "abc"},
		},
		{
			name: "mixed 4",
			in:   []PathFingerprint{"abd", "ab", "ae", "a", "abc"},
			out:  []PathFingerprint{"a", "ab", "abd", "ae", "abc"},
		},
		{
			name: "mixed 5",
			in:   []PathFingerprint{"abd", "ab", "abc", "ae", "a"},
			out:  []PathFingerprint{"a", "ab", "abd", "abc", "ae"},
		},
		{
			name: "stress",
			in:   stressInput,
			out:  stressSorted,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := testdataPathsFromFingerprints(c.in)
			sortStablePartialOrder(paths, func(i, j int) (bool, bool) {
				return subset(string(paths[i].Fingerprint), string(paths[j].Fingerprint))
			})
			actual := fingerprintsFromTestdataPaths(paths)
			assert.Equal(t, c.out, actual)
		})
	}
}
func TestPinnedPolicy(t *testing.T) {
	cases := []struct {
		name   string
		in     []PathFingerprint
		pinned []PathFingerprint
		out    []PathFingerprint
	}{
		{
			name:   "nil",
			in:     nil,
			pinned: []PathFingerprint{},
			out:    []PathFingerprint{},
		},
		{
			name:   "empty",
			in:     []PathFingerprint{},
			pinned: []PathFingerprint{},
			out:    []PathFingerprint{},
		},
		{
			name:   "some",
			in:     []PathFingerprint{"a", "b", "c"},
			pinned: []PathFingerprint{"a"},
			out:    []PathFingerprint{"a"},
		},
		{
			name:   "none",
			in:     []PathFingerprint{"a", "b", "c"},
			pinned: []PathFingerprint{"d"},
			out:    []PathFingerprint{},
		},
		{
			name:   "all",
			in:     []PathFingerprint{"a", "b", "c"},
			pinned: []PathFingerprint{"a", "b", "c"},
			out:    []PathFingerprint{"a", "b", "c"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := testdataPathsFromFingerprints(c.in)
			filtered := Pinned(c.pinned).Filter(paths)
			actual := fingerprintsFromTestdataPaths(filtered)
			assert.Equal(t, c.out, actual)
		})
	}
}

func TestPreferredPolicy(t *testing.T) {
	cases := []struct {
		name      string
		in        []PathFingerprint
		preferred []PathFingerprint
		out       []PathFingerprint
	}{
		{
			name:      "nil",
			in:        nil,
			preferred: []PathFingerprint{},
			out:       []PathFingerprint{},
		},
		{
			name:      "empty",
			in:        []PathFingerprint{},
			preferred: []PathFingerprint{},
			out:       []PathFingerprint{},
		},
		{
			name:      "some",
			in:        []PathFingerprint{"a", "b", "c", "d"},
			preferred: []PathFingerprint{"c", "a"},
			out:       []PathFingerprint{"c", "a", "b", "d"},
		},
		{
			name:      "none",
			in:        []PathFingerprint{"a", "b", "c"},
			preferred: []PathFingerprint{"d"},
			out:       []PathFingerprint{"a", "b", "c"},
		},
		{
			name:      "all",
			in:        []PathFingerprint{"a", "b", "c"},
			preferred: []PathFingerprint{"c", "a", "b"},
			out:       []PathFingerprint{"c", "a", "b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := testdataPathsFromFingerprints(c.in)
			filtered := Preferred{Pinned(c.preferred)}.Filter(paths)
			actual := fingerprintsFromTestdataPaths(filtered)
			assert.Equal(t, c.out, actual)
		})
	}
}

// testdataPathsFromFingerprints creates a path slice only Fingerprints set.
func testdataPathsFromFingerprints(strs []PathFingerprint) []*Path {
	if strs == nil {
		return nil
	}
	paths := make([]*Path, len(strs))
	for i, s := range strs {
		paths[i] = &Path{Fingerprint: s}
	}
	return paths
}

// fingerprintsFromTestdataPaths the Fingerprints fo the paths
func fingerprintsFromTestdataPaths(paths []*Path) []PathFingerprint {
	if paths == nil {
		return nil
	}
	strs := make([]PathFingerprint, len(paths))
	for i, p := range paths {
		strs[i] = p.Fingerprint
	}
	return strs
}

// TestSequencePolicy checks that the glue logic for invoking the pathpol.Sequence
// works correctly. We do not need to extensively test the sequence
// language itself here.
func TestSequencePolicy(t *testing.T) {
	asA := MustParseIA("1-ff00:0:a")
	asB := MustParseIA("1-ff00:0:b")
	asC := MustParseIA("1-ff00:0:c")
	pAB := &Path{
		Metadata: &PathMetadata{
			Interfaces: []PathInterface{
				{IA: asA, IfID: 1},
				{IA: asB, IfID: 2},
			},
		},
	}
	pACB := &Path{
		Metadata: &PathMetadata{
			Interfaces: []PathInterface{
				{IA: asA, IfID: 1},
				{IA: asB, IfID: 2},
				{IA: asB, IfID: 3},
				{IA: asC, IfID: 4},
			},
		},
	}
	cases := []struct {
		name     string
		sequence string
		in       []*Path
		out      []*Path
	}{
		{
			name:     "nil",
			sequence: "0*",
			in:       nil,
			out:      []*Path{},
		},
		{
			name:     "empty",
			sequence: "0*",
			in:       []*Path{},
			out:      []*Path{},
		},
		{
			name:     "any",
			sequence: "0*",
			in:       []*Path{pAB, pACB},
			out:      []*Path{pAB, pACB},
		},
		{
			name:     "only via C",
			sequence: "0* 1-ff00:0:c 0*",
			in:       []*Path{pAB, pACB},
			out:      []*Path{pACB},
		},
		{
			name:     "none",
			sequence: "99+",
			in:       []*Path{pAB, pACB},
			out:      []*Path{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sequence, err := NewSequence(c.sequence)
			assert.NoError(t, err)
			actual := sequence.Filter(c.in)
			assert.Equal(t, c.out, actual)
		})
	}
}

// TestACLPolicy checks that the glue logic for invoking the pathpol.ACL
// works correctly. We do not need to extensively test the sequence
// language itself here.
func TestACLPolicy(t *testing.T) {
	asA := MustParseIA("1-ff00:0:a")
	asB := MustParseIA("1-ff00:0:b")
	asC := MustParseIA("1-ff00:0:c")
	pAB := &Path{
		Metadata: &PathMetadata{
			Interfaces: []PathInterface{
				{IA: asA, IfID: 1},
				{IA: asB, IfID: 2},
			},
		},
	}
	pACB := &Path{
		Metadata: &PathMetadata{
			Interfaces: []PathInterface{
				{IA: asA, IfID: 1},
				{IA: asB, IfID: 2},
				{IA: asB, IfID: 3},
				{IA: asC, IfID: 4},
			},
		},
	}
	cases := []struct {
		name       string
		acl        []string
		in         []*Path
		out        []*Path
		assertFunc assert.ErrorAssertionFunc
	}{
		{
			name:       "nil",
			acl:        nil,
			in:         nil,
			out:        []*Path{},
			assertFunc: assert.Error,
		},
		{
			name:       "empty",
			acl:        []string{},
			in:         nil,
			out:        []*Path{},
			assertFunc: assert.Error,
		},
		{
			name:       "malformed",
			acl:        []string{""},
			in:         nil,
			out:        []*Path{},
			assertFunc: assert.Error,
		},
		{
			name:       "any",
			acl:        []string{"+"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{pAB, pACB},
			assertFunc: assert.NoError,
		},
		{
			name:       "blacklist C",
			acl:        []string{"- 1-ff00:0:c", "+"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{pAB},
			assertFunc: assert.NoError,
		},
		{
			name:       "blacklist B",
			acl:        []string{"- 1-ff00:0:B", "+"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{},
			assertFunc: assert.NoError,
		},
		{
			name:       "blacklist C, white ISD 1",
			acl:        []string{"- 1-ff00:0:C", "+ 1", "-"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{pAB},
			assertFunc: assert.NoError,
		},
		{
			name:       "whitelist only AB",
			acl:        []string{"+ 1-ff00:0:B", "+ 1-ff00:0:A", "-"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{pAB},
			assertFunc: assert.NoError,
		},
		{
			name:       "none",
			acl:        []string{"-"},
			in:         []*Path{pAB, pACB},
			out:        []*Path{},
			assertFunc: assert.NoError,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			acl, err := NewACL(c.acl)
			c.assertFunc(t, err)
			actual := acl.Filter(c.in)
			assert.Equal(t, c.out, actual)
		})
	}
}
