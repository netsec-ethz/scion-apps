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

package scionutil

import (
	"log"
	"testing"

	libaddr "github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func init() {
	hostFilePath = "hosts_test_file"
	hostsFile, err := readHostsFile()
	if err != nil {
		log.Fatal(err)
	}
	parseHostsFile(hostsFile)
}

func TestCount(t *testing.T) {
	count := len(hosts)
	if count != 5 {
		t.Errorf("wrong number of hosts in map, expected: %v, got: %v", 5, count)
	}

	count = len(revHosts["17-ffaa:0:1,[192.168.1.1]"])
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
	ia, l3, err := GetHostByName("host1.2")
	if err != nil {
		t.Error(err)
	}
	addr := &snet.Addr{IA: ia, Host: &libaddr.AppAddr{L3: l3, L4: libaddr.NewL4UDPInfo(0)}}

	expected, err := snet.AddrFromString("17-ffaa:0:1,[192.168.1.1]:0")
	if err != nil {
		panic("This should always work")
	}
	if !addr.EqAddr(expected) {
		t.Errorf("host resolved to wrong address, expected: %q, received: %q", "17-ffaa:0:1,[192.168.1.1]:0", addr)
	}

	// works with IPv6 SCION hosts
	ia, l3, err = GetHostByName("host4")
	if err != nil {
		t.Error(err)
	}
	addr = &snet.Addr{IA: ia, Host: &libaddr.AppAddr{L3: l3, L4: libaddr.NewL4UDPInfo(0)}}

	expected, _ = snet.AddrFromString("20-ffaa:c0ff:ee12,[::ff1:ce00:dead:10cc:baad:f00d]:0")
	if !addr.EqAddr(expected) {
		t.Errorf("host resolved to wrong address, expected: %q, received: %q", "20-ffaa:c0ff:ee12,[::ff1:ce00:dead:10cc:baad:f00d]:0", addr)
	}

	// does not parse commented hosts
	_, _, err = GetHostByName("commented")
	if err == nil {
		t.Error("read commented host")
	}
}

func TestReadAddresses(t *testing.T) {
	addrs, err := GetHostnamesByAddress("18-ffaa:1:2,[10.0.8.10]")
	if err != nil {
		t.Error(err)
	}
	if len(addrs) != 1 || addrs[0] != "host2" {
		t.Errorf("address resolved to wrong hostnames, expected: %v, received: %v", []string{"host2"}, addrs)
	}

	// pass address with port
	addrs, err = GetHostnamesByAddress("17-ffaa:0:1,[192.168.1.1]:40002")
	if err != nil {
		t.Error(err)
	}
	if len(addrs) != 3 || addrs[0] != "host1.1" || addrs[1] != "host1.2" || addrs[2] != "host3" {
		t.Errorf("address resolved to wrong hostnames, expected: %v, received: %v", []string{"host1.1", "host1.2", "host3"}, addrs)
	}

	// pass address with IPv6
	addrs, err = GetHostnamesByAddress("20-ffaa:c0ff:ee12,[0:0:0ff1:ce00:dead:10cc:baad:f00d]")
	if err != nil {
		t.Error(err)
	}
	if len(addrs) != 1 || addrs[0] != "host4" {
		t.Errorf("address resolved to wrong hostnames, expected: %v, received: %v", []string{"host4"}, addrs)
	}
}
