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
	"fmt"
	"net/netip"
	"testing"

	"github.com/scionproto/scion/pkg/connect"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

func TestUDPAddrIsValid(t *testing.T) {
	ia := pan.MustParseIA("1-ff00:0:0")
	iaWildcard := pan.MustParseIA("1-0")
	ip := netip.MustParseAddr("127.0.0.1")
	cases := []struct {
		addr    pan.UDPAddr
		isValid bool
	}{
		{pan.UDPAddr{}, false},
		{pan.UDPAddr{IA: ia}, false},
		{pan.UDPAddr{IP: ip}, false},
		{pan.UDPAddr{IA: ia, IP: ip}, true},
		{pan.UDPAddr{IA: iaWildcard, IP: ip}, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.isValid, c.addr.IsValid(), fmt.Sprintf("%s IsValid?", c.addr))
		// Port does not affect IsValid
		withPort := c.addr.WithPort(8888)
		assert.Equal(t, c.isValid, withPort.IsValid(), fmt.Sprintf("%s IsValid?", withPort))
	}
}

// TestConformanceOfBaseUrl checks that connect.BaseUrl from scionproto always prepends
// "https://" to the result.
func TestConformanceOfBaseUrl(t *testing.T) {
	testCases := []string{
		"1-ff00:0:110,10.0.0.1",
		"1-ff00:0:110,[::1]",
		"1-ff00:0:110,10.0.0.1:31000",
		"1-ff00:0:110,[::1]:31000",
	}
	for _, tc := range testCases {
		addr := mustParseUDPAddr(tc)
		s := connect.BaseUrl(addr)
		require.GreaterOrEqual(t, len(s), 8)
		require.Equal(t, "https://", s[0:8])
	}
}

func TestMangleScionAddr(t *testing.T) {
	testCases := []struct {
		addr     string
		expected string
	}{
		{
			addr:     "1-ff00:0:110,10.0.0.1:31000",
			expected: "scion4-1-ff00-0-110_10-0-0-1:31000",
		},
		{
			addr:     "1-ff00:0:110,[::1]:31000",
			expected: "scion6-1-ff00-0-110_--1:31000",
		},
	}
	for _, tc := range testCases {
		mangled := pan.MangleSCIONAddr(tc.addr)
		require.Equal(t, tc.expected, mangled)
		// Unmangle it.
		unmangled := pan.UnmangleSCIONAddr(mangled)
		// Check the unmangled resolves to the expected address.
		gotAddr, err := pan.ParseUDPAddr(unmangled)
		require.NoError(t, err)
		expectAddr, err := pan.ParseUDPAddr(tc.addr)
		require.NoError(t, err)
		require.Equal(t, expectAddr, gotAddr)
	}
}

func mustParseUDPAddr(s string) *snet.UDPAddr {
	addr, err := snet.ParseUDPAddr(s)
	if err != nil {
		panic(err)
	}
	return addr
}
