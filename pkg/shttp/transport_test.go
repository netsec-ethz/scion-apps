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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
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

			// Check that unmangling the address can be parsed by UDPAddrFromString
			// Only for testcases that have a port set:
			if _, _, err := net.SplitHostPort(u.Host); err != nil {
				continue
			}
			unmangled := unmangleSCIONAddr(u.Host)
			if unmangled != tc.HostPort {
				t.Fatalf("unmangleSCIONAddr('%s') returned different result, actual='%s', expected='%s'", u.Host, unmangled, tc.HostPort)
			}
		}
	}
}

func TestRoundTripper(t *testing.T) {

	// host2 from hosts_test_file
	// XXX we should reorganize this hostname resolution to allow configuring the host_tests_file
	_ = appnet.AddHost("host2", "17-ffaa:0:1,[192.168.1.1]")

	testCases := []struct {
		HostPort string
		Expected string
	}{
		{"host2", "17-ffaa:0:1,192.168.1.1:443"},
		{"host2:80", "17-ffaa:0:1,192.168.1.1:80"},
		{"1-ff00:0:110,127.0.0.1", "1-ff00:0:110,127.0.0.1:443"},
		{"1-ff00:0:110,127.0.0.1:80", "1-ff00:0:110,127.0.0.1:80"},
		{"1-ff00:0:110,::1", "1-ff00:0:110,[::1]:443"},
		{"1-ff00:0:110,[::1]", "1-ff00:0:110,[::1]:443"},
		{"1-ff00:0:110,[::1]:80", "1-ff00:0:110,[::1]:80"},
	}

	urlPatterns := hostURLPatterns()

	// We replace the actual dial function of the roundtripper with this function that only
	// checks wether the address can be successfully unmangled and resolved.
	// expected will be set in the test loop, below
	var expected string
	testDial := func(network, address string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {
		unmangled := unmangleSCIONAddr(address)
		resolvedAddr, err := appnet.ResolveUDPAddr(unmangled)
		if err != nil {
			t.Fatalf("unexpected error when resolving address '%s' in roundtripper: %s", unmangled, err)
		}
		actual := resolvedAddr.String()
		if actual != expected {
			t.Fatalf("unexpected address resolved in roundtripper, actual='%s', expected='%s'", actual, expected)
		}
		return nil, errors.New("just a test")
	}

	rt := NewRoundTripper(nil, nil)
	rt.(*roundTripper).rt.Dial = testDial
	c := &http.Client{Transport: rt}

	for _, tc := range testCases {
		for _, urlPattern := range urlPatterns {
			expected = tc.Expected

			url := fmt.Sprintf(urlPattern, tc.HostPort)
			_, err := c.Get(MangleSCIONAddrURL(url))
			if err == nil {
				panic("unexpected success!")
			} else if !strings.Contains(err.Error(), "just a test") {
				t.Fatalf("unexpected error: %s", err)
			}
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
