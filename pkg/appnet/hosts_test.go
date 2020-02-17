// Copyright 2018 ETH Zurich
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
// limitations under the License.package main

package appnet

import (
	"fmt"
	"sort"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func init() {
	// Trick into thinking that /etc/hosts is already loaded
	loadHostsOnce.Do(func() {
		hostsTableInstance = loadHostsFile("hosts_test_file")
	})
}

func TestCount(t *testing.T) {
	count := len(hosts().byName)
	if count != 5 {
		t.Errorf("wrong number of hosts in map, expected: %v, got: %v", 5, count)
	}

	count = len(hosts().byAddr["17-ffaa:0:1,[192.168.1.1]"])
	if count != 3 {
		t.Errorf("wrong number of addresses in list, expected: %v, got: %v", 3, count)
	}
}

func TestAddingHost(t *testing.T) {
	host := "host5"
	addr := "20-ffaa:3:4,[12.34.56.0]"

	// can add new host/addr
	err := AddHost(host, addr)
	if err != nil {
		t.Error(err)
	}

	// cannot add host twice
	err = AddHost(host, addr)
	if err == nil {
		t.Error("added host with same name twice")
	}

	// can add different host with same address
	host = "host6"
	err = AddHost(host, addr)
	if err != nil {
		t.Error(err)
	}
}

func TestReadHosts(t *testing.T) {
	cases := []struct {
		name     string
		expected snet.SCIONAddress
	}{
		{"host1.1", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host1.2", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host2", mustParse("18-ffaa:1:2,[10.0.8.10]")},
		{"host3", mustParse("17-ffaa:0:1,[192.168.1.1]")},
		{"host4", mustParse("20-ffaa:c0ff:ee12,[::ff1:ce00:dead:10cc:baad:f00d]")},
		{"commented", snet.SCIONAddress{}},
		{"dummy1", snet.SCIONAddress{}},
		{"dummy2", snet.SCIONAddress{}},
	}

	for _, c := range cases {
		actual, err := GetHostByName(c.name)
		if c.expected.Host == nil {
			if err == nil {
				t.Errorf("no result expected for '%s', got %v", c.name, actual)
			}
		} else {
			if err != nil {
				t.Error(err)
			}
			if c.expected.IA != actual.IA || !c.expected.Host.Equal(actual.Host) {
				t.Errorf("wrong result for '%s', expected %v, got %v", c.name, c.expected, actual)
			}
		}
	}
}

func TestReadAddresses(t *testing.T) {

	equalSet := func(a, b []string) bool {
		sort.Strings(a)
		sort.Strings(b)
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	cases := []struct {
		addr     snet.SCIONAddress
		expected []string
	}{
		{mustParse("18-ffaa:1:2,[10.0.8.10]"), []string{"host2"}},
		{mustParse("17-ffaa:0:1,[192.168.1.1]"), []string{"host1.1", "host1.2", "host3"}},
		{mustParse("20-ffaa:c0ff:ee12,[::ff1:ce00:dead:10cc:baad:f00d]"), []string{"host4"}},
		{mustParse("1-ff00:0:1,[1.0.0.1]"), nil},
	}

	for _, c := range cases {
		names, err := GetHostnamesByAddress(c.addr)
		if c.expected == nil {
			if err == nil {
				t.Errorf("no result expected for %v, got %v", c.addr, names)
			}
		} else {
			if err != nil {
				t.Error(err)
			}
			if !equalSet(names, c.expected) {
				t.Errorf("wrong result for %v, expected %v, got %v", c.addr, c.expected, names)
			}
		}
	}
}

func mustParse(address string) snet.SCIONAddress {
	a, err := snet.UDPAddrFromString(address)
	if err != nil {
		panic(fmt.Sprintf("test input must parse %s", err))
	}
	return snet.SCIONAddress{IA: a.IA, Host: addr.HostFromIP(a.Host.IP)}
}

func TestSplitHostPort(t *testing.T) {
	type testCase struct {
		input string
		host  string
		port  string
		err   bool
	}
	cases := []testCase{
		{"1-ff00:0:0,[1.1.1.1]:80", "1-ff00:0:0,[1.1.1.1]", "80", false},
		{"1-ff00:0:0,[::]:80", "1-ff00:0:0,[::]", "80", false},
		{"foo:80", "foo", "80", false},
		{"www.example.com:666", "www.example.com", "666", false},
		{":foo:666", "", "", true},
		{"1-ff00:0:0,[1.1.1.1]", "", "", true},
		{"1-ff00:0:0,[::]", "", "", true},
		{"foo", "", "", true},
	}
	for _, c := range cases {
		host, port, err := SplitHostPort(c.input)
		if err != nil && !c.err {
			t.Errorf("Failed, but shouldn't have. input: %s, error: %s", c.input, err)
		} else if err == nil && c.err {
			t.Errorf("Did not fail, but should have. input: %s, host: %s, port: %s", c.input, host, port)
		} else if err != nil {
			if host != c.host || port != c.port {
				t.Errorf("Bad result. input: %s, host: %s, port: %s", c.input, host, port)
			}
		}
	}
}
