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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPUDPHeaderBuilder(t *testing.T) {

	cases := []struct {
		name     string
		expected []byte
		run      func(b *pudpHeaderBuilder)
	}{
		{
			name:     "empty",
			run:      func(b *pudpHeaderBuilder) {},
			expected: nil,
		},
		{
			name: "race",
			run: func(b *pudpHeaderBuilder) {
				b.race(1)
			},
			expected: []byte{
				byte(pudpHeaderRace), 0x0, 0x1,
			},
		},
		{
			name: "race, ping",
			run: func(b *pudpHeaderBuilder) {
				b.race(1)
				b.ping(1337)
			},
			expected: []byte{
				byte(pudpHeaderRace), 0x0, 0x1,
				byte(pudpHeaderPing), 0x5, 0x39,
			},
		},
		{
			name: "identify",
			run: func(b *pudpHeaderBuilder) {
				b.identify()
			},
			expected: []byte{
				byte(pudpHeaderIdentify),
			},
		},
		{
			name: "ping",
			run: func(b *pudpHeaderBuilder) {
				b.ping(1337)
			},
			expected: []byte{byte(pudpHeaderPing), 0x5, 0x39},
		},
		{
			name: "pong",
			run: func(b *pudpHeaderBuilder) {
				b.pong(1337)
			},
			expected: []byte{byte(pudpHeaderPong), 0x5, 0x39},
		},
		{
			name: "me 2",
			run: func(b *pudpHeaderBuilder) {
				b.me([]IfID{7, 8})
			},
			expected: []byte{
				byte(pudpHeaderMe), 2, 0x0, 0x7, 0x0, 0x8,
			},
		},
		{
			name: "identify, ping, race",
			run: func(b *pudpHeaderBuilder) {
				b.identify()
				b.ping(1337)
				b.race(1)
			},
			expected: []byte{
				byte(pudpHeaderIdentify),
				byte(pudpHeaderPing), 0x5, 0x39,
				byte(pudpHeaderRace), 0x0, 0x1,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := &pudpHeaderBuilder{}
			c.run(b)
			assert.Equal(t, c.expected, b.Bytes())
		})
	}
}

func TestPUDPHeaderParser(t *testing.T) {
	cases := []struct {
		name     string
		buf      []byte
		err      bool
		expected string
	}{
		{
			name:     "nil",
			buf:      nil,
			expected: "",
		},
		{
			name:     "empty",
			buf:      []byte{},
			expected: "",
		},
		{
			name:     "payload",
			buf:      []byte{byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o'},
			expected: "pld[hello]",
		},
		{
			name: "race, pld",
			buf: []byte{
				byte(pudpHeaderRace), 0x0, 0x1,
				byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o',
			},
			expected: "race[1];pld[hello]",
		},
		{
			name: "race, ping, pld",
			buf: []byte{
				byte(pudpHeaderRace), 0x0, 0x1,
				byte(pudpHeaderPing), 0x5, 0x39,
				byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o',
			},
			expected: "race[1];ping[1337];pld[hello]",
		},
		{
			name:     "ping",
			buf:      []byte{byte(pudpHeaderPing), 0x5, 0x39},
			expected: "ping[1337];",
		},
		{
			name: "ping bad",
			buf:  []byte{byte(pudpHeaderPing), 0x5},
			err:  true,
		},
		{
			name:     "pong",
			buf:      []byte{byte(pudpHeaderPong), 0x5, 0x39},
			expected: "pong[1337];",
		},
		{
			name: "pong bad",
			buf:  []byte{byte(pudpHeaderPong), 0x5},
			err:  true,
		},
		{
			name: "identify, ping, race, pld",
			buf: []byte{
				byte(pudpHeaderIdentify),
				byte(pudpHeaderPing), 0x5, 0x39,
				byte(pudpHeaderRace), 0x0, 0x1,
				byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o',
			},
			expected: "identify;ping[1337];race[1];pld[hello]",
		},
		{
			name: "ping, identify, race, pld",
			buf: []byte{
				byte(pudpHeaderPing), 0x5, 0x39,
				byte(pudpHeaderIdentify),
				byte(pudpHeaderRace), 0x0, 0x1,
				byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o',
			},
			expected: "ping[1337];identify;race[1];pld[hello]",
		},
		{
			name: "me 0",
			buf: []byte{
				byte(pudpHeaderMe), 0,
			},
			expected: "me[];",
		},
		{
			name: "me 1",
			buf: []byte{
				byte(pudpHeaderMe), 1, 0x0, 0x7,
			},
			expected: "me[7];",
		},
		{
			name: "me 2",
			buf: []byte{
				byte(pudpHeaderMe), 2, 0x0, 0x7, 0x0, 0x8,
			},
			expected: "me[7 8];",
		},
		{
			name: "me 3, payload",
			buf: []byte{
				byte(pudpHeaderMe), 3, 0x0, 0x7, 0x0, 0x8, 0x2, 0x1,
				byte(pudpHeaderPayload), 'h', 'e', 'l', 'l', 'o',
			},
			expected: "me[7 8 513];pld[hello]",
		},
		{
			name: "me bad 1.5",
			buf: []byte{
				byte(pudpHeaderMe), 2, 0x0, 0x7, 0x0,
			},
			err: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := &testPudpHeaderVisitor{}
			err := pudpParseHeader(c.buf, v)
			if c.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.expected, v.s)
			}
		})
	}
}

// testPudpHeaderVisitor parses a header into a string representation (only!)
// suitable for comparison in a test.
type testPudpHeaderVisitor struct {
	s string
}

func (t *testPudpHeaderVisitor) payload(b []byte) {
	t.s += fmt.Sprintf("pld[%s]", b)
}

func (t *testPudpHeaderVisitor) race(seq uint16) {
	t.s += fmt.Sprintf("race[%d];", seq)
}

func (t *testPudpHeaderVisitor) ping(seq uint16) {
	t.s += fmt.Sprintf("ping[%d];", seq)
}

func (t *testPudpHeaderVisitor) pong(seq uint16) {
	t.s += fmt.Sprintf("pong[%d];", seq)
}

func (t *testPudpHeaderVisitor) identify() {
	t.s += "identify;"
}

func (t *testPudpHeaderVisitor) me(ifids []IfID) {
	t.s += fmt.Sprintf("me%v;", ifids)
}
