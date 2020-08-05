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

package appnet

import (
	"fmt"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

const hostsTestFile = "hosts_test_file"

func TestCount(t *testing.T) {

	hosts, err := loadHostsFile(hostsTestFile)
	if err != nil {
		t.Fatal("error loading test file", err)
	}

	if len(hosts) != 5 {
		t.Errorf("wrong number of hosts in map, expected: %v, got: %v", 5, len(hosts))
	}
}

func TestHostsfileResolver(t *testing.T) {
	resolver := &hostsfileResolver{hostsTestFile}

	cases := []testCase{
		{"host1.1", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host1.2", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host2", mustParse("18-ffaa:1:2,[10.0.8.10]")},
		{"host3", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host4", mustParse("20-ffaa:c0ff:ee12,[::ff1:ce00:dead:10cc:baad:f00d]")},
		{"commented", nil},
		{"dummy1", nil},
		{"dummy2", nil},
		{"dummy3", nil},
		{"foobar", nil},
	}
	testResolver(t, resolver, cases)
}

func TestHostsfileResolverNonexisting(t *testing.T) {
	resolver := &hostsfileResolver{"non_existing_hosts_file"}
	testResolver(t, resolver, []testCase{{"something", nil}})
}

func TestResolverList(t *testing.T) {
	primary := map[string]*snet.SCIONAddress{
		"foo": mustParse("1-ff00:0:f00,[192.0.2.1]"),
		"bar": mustParse("1-ff00:0:ba3,[192.0.2.1]"),
	}
	secondary := map[string]*snet.SCIONAddress{
		"bar": mustParse("1-ff00:0:ba3,[2001:db8:ffff:ffff:ffff:ffff:baad:f00d]"), // shadowed by bar in primary
		"baz": mustParse("1-ff00:0:ba5,[192.0.2.1]"),
	}
	resolver := ResolverList{
		dummyResolver{primary},
		dummyResolver{secondary},
	}

	cases := []testCase{
		{"foo", mustParse("1-ff00:0:f00,[192.0.2.1]")},
		{"bar", mustParse("1-ff00:0:ba3,[192.0.2.1]")},
		{"baz", mustParse("1-ff00:0:ba5,[192.0.2.1]")},
		{"boo", nil},
	}
	testResolver(t, resolver, cases)
}

type dummyResolver struct {
	hosts map[string]*snet.SCIONAddress
}

func (r dummyResolver) Resolve(name string) (*snet.SCIONAddress, error) {
	if h, ok := r.hosts[name]; ok {
		return h, nil
	} else {
		return nil, &HostNotFoundError{Host: name}
	}
}

type testCase struct {
	name     string
	expected *snet.SCIONAddress
}

func testResolver(t *testing.T, resolver Resolver, cases []testCase) {
	for _, c := range cases {
		actual, err := resolver.Resolve(c.name)
		if c.expected == nil {
			if err == nil {
				t.Errorf("no result expected for '%s', got %v", c.name, actual)
			} else if _, ok := err.(*HostNotFoundError); !ok {
				t.Errorf("expected HostNotFoundError, got %v", err)
			}
		} else {
			if err != nil {
				t.Error(err)
			}
			if actual == nil {
				t.Errorf("no result for '%s', expected %v", c.name, c.expected)
			} else if c.expected.IA != actual.IA || !c.expected.Host.Equal(actual.Host) {
				t.Errorf("wrong result for '%s', expected %v, got %v", c.name, c.expected, actual)
			}
		}
	}
}

func mustParse(address string) *snet.SCIONAddress {
	a, err := snet.ParseUDPAddr(address)
	if err != nil {
		panic(fmt.Sprintf("test input must parse %s", err))
	}
	return &snet.SCIONAddress{IA: a.IA, Host: addr.HostFromIP(a.Host.IP)}
}
