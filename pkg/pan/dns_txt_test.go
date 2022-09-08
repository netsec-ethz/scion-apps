// Copyright 2022 ETH Zurich
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
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDNSResolver(t *testing.T) {
	cases := []struct {
		name      string
		assertErr assert.ErrorAssertionFunc
		expected  scionAddr
	}{
		{"example.com", assert.NoError, mustParse("1-ff00:0:f00,[192.0.2.1]")},
		{"example.net", assert.NoError, mustParse("1-ff00:0:ba5,[192.0.2.1]")},
		{"dummy4", assertErrHostNotFound, scionAddr{}},
		{"barbaz", assertErrHostNotFound, scionAddr{}},
	}
	var m *mockResolver
	resolver := &dnsResolver{res: m}
	for _, c := range cases {
		actual, err := resolver.Resolve(context.TODO(), c.name)
		if !c.assertErr(t, err) {
			continue
		}
		assert.Equal(t, c.expected, actual)
	}
}

func TestDNSResolverInvalid(t *testing.T) {
	r := &dnsResolver{res: nil}
	_, err := r.Resolve(context.TODO(), "example.com")
	assert.Error(t, err)
}

type mockResolver struct {
	net.Resolver
}

// LookupTXT mocks requesting the DNS TXT records for the given domain name.
func (r *mockResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	v, ok := map[string][]string{
		"example.com.": {
			"NS=ns74430548",
			"doodle-site-verification=t4SRCkhsSDk_Ec9BPAr4xWQvYqoYJSLuoMmWLBdKqS0",
			"scion=1-ff00:0:f00,[192.0.2.1]",
		},
		"example.net.": {
			"scion=1-ff00:0:ba5,[192.0.2.1]",
			"BOOM_verify_3ovqPKkST76TzF2c7b13YA",
			"v=spf1 include:_id.example.net ip4:192.0.2.38 ip4:192.0.2.197 ~all",
		},
	}[name]
	if !ok {
		return nil, &net.DNSError{IsNotFound: true}
	}
	return v, nil
}
