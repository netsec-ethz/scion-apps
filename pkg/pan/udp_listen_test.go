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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathsMRU(t *testing.T) {
	const maxSize = 3
	cases := []struct {
		name   string
		before []string
		insert string
		after  []string
	}{
		{
			name:   "nil",
			before: nil,
			insert: "a",
			after:  []string{"a"},
		},
		{
			name:   "empty",
			before: []string{},
			insert: "a",
			after:  []string{"a"},
		},
		{
			name:   "new, not full",
			before: []string{"a", "b"},
			insert: "c",
			after:  []string{"c", "a", "b"},
		},
		{
			name:   "existing, not full",
			before: []string{"a", "b"},
			insert: "b",
			after:  []string{"b", "a"},
		},
		{
			name:   "new, full",
			before: []string{"a", "b", "c"},
			insert: "d",
			after:  []string{"d", "a", "b"},
		},
		{
			name:   "existing, full, first",
			before: []string{"a", "b", "c"},
			insert: "a",
			after:  []string{"a", "b", "c"},
		},
		{
			name:   "existing, full, middle",
			before: []string{"a", "b", "c"},
			insert: "b",
			after:  []string{"b", "a", "c"},
		},
		{
			name:   "existing, full, last",
			before: []string{"a", "b", "c"},
			insert: "c",
			after:  []string{"c", "a", "b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := testdataPathsFromStrings(c.before)
			l := pathsMRU(paths)
			l.insert(&Path{Fingerprint: PathFingerprint(c.insert)}, maxSize)
			actual := stringsFromTestdataPaths(l)
			assert.Equal(t, c.after, actual)
		})
	}
}
