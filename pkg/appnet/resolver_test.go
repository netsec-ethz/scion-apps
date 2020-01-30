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
	if resolveRains != nil {
		fmt.Println("haz rains")
	} else {
		fmt.Println("no haz rains")

	}
	testResolver(t, &hostsfileResolver{hostsTestFile})
}

func TestResolverList(t *testing.T) {
	resolvers := resolverList{
		&hostsfileResolver{"non_existing_hosts_file"},
		&hostsfileResolver{hostsTestFile},
		&hostsfileResolver{"another_non_existing_host_file"},
	}
	testResolver(t, resolvers)
}

// testResolver checks that exactly the names in the hosts_test_file are
// resolved
func testResolver(t *testing.T, resolver Resolver) {

	cases := []struct {
		name     string
		expected *snet.SCIONAddress
	}{
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

	for _, c := range cases {
		actual, err := resolver.Resolve(c.name)
		if c.expected == nil {
			if err == nil {
				t.Errorf("no result expected for '%s', got %v", c.name, actual)
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
