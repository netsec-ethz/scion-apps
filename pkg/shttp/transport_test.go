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

package shttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

func TestMangleSCIONAddrURL(t *testing.T) {
	testCases := []struct {
		HostPort string
		Expected string
	}{
		{"foo", "foo"},
		{"foo:80", "foo:80"},
		{"1-ff00:0:110,127.0.0.1", "[1-ff00:0:110,127.0.0.1]"},
		{"1-ff00:0:110,127.0.0.1:80", "[1-ff00:0:110,127.0.0.1]:80"},
		{"1-ff00:0:110,::1", "[1-ff00:0:110,::1]"},
		{"1-ff00:0:110,[::1]", "[1-ff00:0:110,::1]"},
		{"1-ff00:0:110,[::1]:80", "[1-ff00:0:110,::1]:80"},
	}

	urlPatterns := hostURLPatterns()

	for _, tc := range testCases {
		for _, urlPattern := range urlPatterns {
			mangled := MangleSCIONAddrURL(fmt.Sprintf(urlPattern, tc.HostPort))
			expected := fmt.Sprintf(urlPattern, tc.Expected)
			if mangled != expected {
				t.Fatalf("MangleSCIONAddrURL returned different result, actual='%s', expected='%s'", mangled, expected)
			}
			// Now attempt to parse this URL. If this fails, the expected test results are broken.
			u, err := url.Parse(mangled)
			if err != nil {
				t.Fatalf("MangleSCIONAddrURL returned URL that cannot be parsed: %s", err)
			}

			// Check that unmangling the address can be parsed by ParseUDPAddr
			// Only for testcases that have a port set:
			if _, _, err := net.SplitHostPort(u.Host); err != nil {
				continue
			}
			unmangled := pan.UnmangleSCIONAddr(u.Host)
			if unmangled != tc.HostPort {
				t.Fatalf("UnmangleSCIONAddr('%s') returned different result, actual='%s', expected='%s'", u.Host, unmangled, tc.HostPort)
			}
		}
	}
}

func TestRoundTripper(t *testing.T) {
	testCases := []struct {
		HostPort string
		Expected string
	}{
		{"host", "1-ff00:0:1,192.0.2.1:443"},
		{"host:80", "1-ff00:0:1,192.0.2.1:80"},
		{"1-ff00:0:110,127.0.0.1", "1-ff00:0:110,127.0.0.1:443"},
		{"1-ff00:0:110,127.0.0.1:80", "1-ff00:0:110,127.0.0.1:80"},
		{"1-ff00:0:110,::1", "1-ff00:0:110,[::1]:443"},
		{"1-ff00:0:110,[::1]", "1-ff00:0:110,[::1]:443"},
		{"1-ff00:0:110,[::1]:80", "1-ff00:0:110,[::1]:80"},
	}

	urlPatterns := hostURLPatterns()
	errJustATest := errors.New("just a test")

	// We replace the actual dial function of the roundtripper with this function that only
	// checks wether the address can be successfully unmangled and resolved.
	// expected will be set in the test loop, below
	var expected string
	testDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		// The actual Dialer.DialContext does
		//  remote, err := pan.ResolveUDPAddr(pan.UnmangleSCIONAddr(addr))
		// We mock pan.ResolveUDPAddr here; don't want to rely on hosts files etc
		// for this test.
		unmangled := pan.UnmangleSCIONAddr(addr)
		if strings.HasPrefix(unmangled, "host") {
			_, err := pan.ParseUDPAddr(unmangled)
			require.Error(t, err)
			hostStr, portStr, err := net.SplitHostPort(unmangled)
			require.NoError(t, err)
			port, err := strconv.Atoi(portStr)
			require.NoError(t, err)
			require.Equal(t, hostStr, "host")
			hostIA, err := pan.ParseIA("1-ff00:0:1")
			require.NoError(t, err)
			hostIP := netaddr.MustParseIP("192.0.2.1")
			remote := pan.UDPAddr{IA: hostIA, IP: hostIP, Port: uint16(port)}
			assert.Equal(t, expected, remote.String())
		} else {
			remote, err := pan.ParseUDPAddr(unmangled)
			require.NoError(t, err)
			assert.Equal(t, expected, remote.String())
		}
		return nil, errJustATest
	}

	c := &http.Client{
		Transport: &http.Transport{
			DialContext: testDial,
		},
	}

	for _, tc := range testCases {
		for _, urlPattern := range urlPatterns {
			expected = tc.Expected

			url := fmt.Sprintf(urlPattern, tc.HostPort)
			_, err := c.Get(MangleSCIONAddrURL(url)) //nolint:bodyclose
			assert.ErrorIs(t, err, errJustATest)
		}
	}
}

// hostURLPatterns returns a slice of URL patterns in which a host can be inserted
func hostURLPatterns() []string {
	return []string{
		"https://%s",
		"https://user@%s",
		"https://%s/hello",
		"https://user@%s/hello",
		"https://%s?boo=bla",
		"https://user@%s?boo=bla",
		"https://%s/hello?boo=bla",
		"https://user@%s/hello?boo=bla",
	}
}
