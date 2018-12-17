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
	if count != 3 {
		t.Errorf("wrong number of hosts in map, expected: %v, got: %v", 3, count)
	}

	count = len(revHosts["17-ffaa:0:1,[192.168.1.1]"])
	if count != 2 {
		t.Errorf("wrong number of address in list, expected: %v, got: %v", 3, count)
	}
}

func TestAddingHost(t *testing.T) {
	host := "host4"
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
	host = "host5"
	err = AddHost(host, addr)
	if err != nil {
		t.Error(err)
	}
}

func TestReadHosts(t *testing.T) {
	addr, err := GetHostByName("host1", "555")
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "17-ffaa:0:1,[192.168.1.1]:555" {
		t.Errorf("host resolved to wrong address, expected: %q, received: %q", "17-ffaa:0:1,[192.168.1.1]:555", addr)
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
	if len(addrs) != 2 || addrs[0] != "host1" || addrs[1] != "host3" {
		t.Errorf("address resolved to wrong hostnames, expected: %v, received: %v", []string{"host1", "host3"}, addrs)
	}
}
