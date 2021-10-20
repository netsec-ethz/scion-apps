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
	"time"

	"github.com/scionproto/scion/go/lib/slayers/path/scion"
	"github.com/stretchr/testify/assert"
)

func TestPathString(t *testing.T) {
	asA := IA{I: 1, A: 0xff00_0000_000a}
	asB := IA{I: 1, A: 0xff00_0000_000b}
	asC := IA{I: 1, A: 0xff00_0000_000c}

	ifA1 := PathInterface{IA: asA, IfID: 1}
	ifB1 := PathInterface{IA: asB, IfID: 11}
	ifB2 := PathInterface{IA: asB, IfID: 22}
	ifC2 := PathInterface{IA: asC, IfID: 2}

	const testFingerprint = "test-fingerprint"

	cases := []struct {
		name       string
		interfaces []PathInterface
		expected   string
	}{
		{
			name:       "no metadata",
			interfaces: nil,
			expected:   "0-0 0-0 " + testFingerprint,
		},
		{
			name:       "empty",
			interfaces: []PathInterface{},
			expected:   "",
		},
		{
			name:       "one hop",
			interfaces: []PathInterface{ifA1, ifB1},
			expected:   "1-ff00:0:a 1>11 1-ff00:0:b",
		},
		{
			name:       "two hops",
			interfaces: []PathInterface{ifA1, ifB1, ifB2, ifC2},
			expected:   "1-ff00:0:a 1>11 1-ff00:0:b 22>2 1-ff00:0:c",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m *PathMetadata
			if c.interfaces != nil {
				m = &PathMetadata{
					Interfaces: c.interfaces,
				}
			}
			p := &Path{Metadata: m, Fingerprint: testFingerprint}
			actual := p.String()
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestInterfacesFromDecoded(t *testing.T) {
	// Not a great test case...
	rawPath := []byte("\x00\x00\x20\x80\x00\x00\x01\x11\x00\x00\x01\x00\x01\x00\x02\x22\x00\x00" +
		"\x01\x00\x00\x3f\x00\x01\x00\x00\x01\x02\x03\x04\x05\x06\x00\x3f\x00\x03\x00\x02\x01\x02\x03" +
		"\x04\x05\x06\x00\x3f\x00\x00\x00\x02\x01\x02\x03\x04\x05\x06\x00\x3f\x00\x01\x00\x00\x01\x02" +
		"\x03\x04\x05\x06")

	sp := scion.Decoded{}
	err := sp.DecodeFromBytes(rawPath)
	if err != nil {
		panic(err)
	}
	ifaces := interfaceIDsFromDecoded(sp)
	expected := []IfID{1, 2, 2, 1}
	assert.Equal(t, ifaces, expected)
}

func TestLowerLatency(t *testing.T) {
	unknown := time.Duration(0)

	asA := IA{I: 1, A: 1}
	asB := IA{I: 1, A: 2}
	asC := IA{I: 1, A: 3}

	ifA1 := PathInterface{IA: asA, IfID: 1}
	ifB1 := PathInterface{IA: asB, IfID: 1}
	ifB2 := PathInterface{IA: asB, IfID: 2}
	ifC2 := PathInterface{IA: asC, IfID: 2}
	ifA3 := PathInterface{IA: asA, IfID: 3}
	ifC3 := PathInterface{IA: asC, IfID: 3}
	ifB4 := PathInterface{IA: asB, IfID: 4}
	ifC4 := PathInterface{IA: asC, IfID: 4}

	ifseqAC := []PathInterface{ifA3, ifC3}
	ifseqABC := []PathInterface{ifA1, ifB1, ifB2, ifC2}
	ifseqAB4C := []PathInterface{ifA1, ifB1, ifB4, ifC4}

	cases := []struct {
		name            string
		a, b            PathMetadata
		less, ok, equal bool // expected result
	}{
		{
			name:  "empty, equal",
			less:  false,
			ok:    true,
			equal: true,
		},
		{
			name: "all known, less",
			less: true,
			ok:   true,
			a: PathMetadata{
				Interfaces: ifseqAC,
				Latency:    []time.Duration{1},
			},
			b: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{1, 1, 1},
			},
		},
		{
			name:  "all known, equal",
			less:  false,
			ok:    true,
			equal: true,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{1, 1, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAC,
				Latency:    []time.Duration{3},
			},
		},
		{
			name: "all known vs all unknown",
			ok:   false,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{1, 1, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAC,
				Latency:    []time.Duration{unknown},
			},
		},
		{
			name: "some unknowns, can only be more",
			less: false,
			ok:   true,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{1, unknown, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAC,
				Latency:    []time.Duration{1},
			},
		},
		{
			name: "same unknowns, less",
			less: true,
			ok:   true,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{unknown, 1, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAB4C,
				Latency:    []time.Duration{unknown, 1, 2},
			},
		},
		{
			name:  "same unknowns, equal",
			less:  false,
			ok:    true,
			equal: true,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{unknown, 1, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAB4C,
				Latency:    []time.Duration{unknown, 1, 1},
			},
		},
		{
			name: "fewer unknowns, less",
			less: true,
			ok:   true,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{unknown, 1, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAB4C,
				Latency:    []time.Duration{unknown, unknown, 3},
			},
		},
		{
			name: "unknown unknowns",
			ok:   false,
			a: PathMetadata{
				Interfaces: ifseqABC,
				Latency:    []time.Duration{unknown, unknown, 1},
			},
			b: PathMetadata{
				Interfaces: ifseqAB4C,
				Latency:    []time.Duration{unknown, unknown, 3},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			less, ok := c.a.LowerLatency(&c.b)
			type result struct {
				less, ok bool
			}
			assert.Equal(t, result{c.less, c.ok}, result{less: less, ok: ok})

			// check reverse;
			expectedReverseLess := !c.less
			if !c.ok || c.equal {
				expectedReverseLess = false
			}
			rLess, rOk := c.b.LowerLatency(&c.a)
			assert.Equal(t, result{expectedReverseLess, c.ok}, result{rLess, rOk})
		})
	}
}
