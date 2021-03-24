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
	stressSorted := make([]string, 400)
	sb := strings.Builder{}
	for i := range stressSorted {
		stressSorted[i] = sb.String()
		sb.WriteRune(rune(i))
	}
	stressInput := append([]string{}, stressSorted...)
	rand.Shuffle(len(stressInput), func(i, j int) {
		stressInput[i], stressInput[j] = stressInput[j], stressInput[i]
	})

	// test the sorting
	cases := []struct {
		name string
		in   []string
		out  []string
	}{
		{
			name: "nil",
			in:   nil,
			out:  nil,
		},
		{
			name: "empty",
			in:   []string{},
			out:  []string{},
		},
		{
			name: "only incomparable",
			in:   []string{"a", "b"},
			out:  []string{"a", "b"},
		},
		{
			name: "only comparable",
			in:   []string{"ab", "abc", "a"},
			out:  []string{"a", "ab", "abc"},
		},
		// mixed 1-5 inputs contain the same sets but in different order.
		// The sorting is supposed to be *stable*.
		{
			name: "mixed 1",
			in:   []string{"ae", "ab", "abc", "abd", "a"},
			out:  []string{"a", "ae", "ab", "abc", "abd"},
		},
		{
			name: "mixed 2",
			in:   []string{"ab", "ae", "abc", "abd", "a"},
			out:  []string{"a", "ab", "ae", "abc", "abd"},
		},
		{
			name: "mixed 3",
			in:   []string{"ab", "ae", "abd", "a", "abc"},
			out:  []string{"a", "ab", "ae", "abd", "abc"},
		},
		{
			name: "mixed 4",
			in:   []string{"abd", "ab", "ae", "a", "abc"},
			out:  []string{"a", "ab", "abd", "ae", "abc"},
		},
		{
			name: "mixed 5",
			in:   []string{"abd", "ab", "abc", "ae", "a"},
			out:  []string{"a", "ab", "abd", "abc", "ae"},
		},
		{
			name: "stress",
			in:   stressInput,
			out:  stressSorted,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := testdataPathsFromStrings(c.in)
			sortStablePartialOrder(paths, func(i, j int) (bool, bool) {
				return subset(string(paths[i].Fingerprint), string(paths[j].Fingerprint))
			})
			actual := stringsFromTestdataPaths(paths)
			assert.Equal(t, c.out, actual)
		})
	}
}

// testdataPathsFromStrings creates a path slice with the strings hidden away
// in the fingerprint.
func testdataPathsFromStrings(strs []string) []*Path {
	if strs == nil {
		return nil
	}
	paths := make([]*Path, len(strs))
	for i, s := range strs {
		paths[i] = &Path{Fingerprint: PathFingerprint(s)}
	}
	return paths
}

// stringsFromTestdataPaths extracts the strings hidden in the paths by
// testdataPathsFromStrings
func stringsFromTestdataPaths(paths []*Path) []string {
	if paths == nil {
		return nil
	}
	strs := make([]string, len(paths))
	for i, p := range paths {
		strs[i] = string(p.Fingerprint)
	}
	return strs
}
