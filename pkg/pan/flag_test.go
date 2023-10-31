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

package pan_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

func TestParseOptionalIPPort(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		assertErr assert.ErrorAssertionFunc
		expected  netip.AddrPort
	}{
		{
			name:      "empty",
			input:     "",
			assertErr: assert.NoError,
			expected:  netip.AddrPort{},
		},
		{
			name:      "port 0",
			input:     ":0",
			assertErr: assert.NoError,
			expected:  netip.AddrPort{},
		},
		{
			name:      "port",
			input:     ":8888",
			assertErr: assert.NoError,
			expected:  netip.AddrPortFrom(netip.Addr{}, 8888),
		},
		{
			name:      "ipv4 and port",
			input:     "127.0.0.1:8888",
			assertErr: assert.NoError,
			expected:  netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 8888),
		},
		{
			name:      "ipv6 and port",
			input:     "[::1]:8888",
			assertErr: assert.NoError,
			expected:  netip.AddrPortFrom(netip.MustParseAddr("::1"), 8888),
		},
		{
			name:      "ipv4 only",
			input:     "127.0.0.1",
			assertErr: assert.Error,
		},
		{
			name:      "ipv6 only",
			input:     "::1",
			assertErr: assert.Error,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := pan.ParseOptionalIPPort(c.input)
			if !c.assertErr(t, err) {
				return
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}
