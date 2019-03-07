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
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	libaddr "github.com/scionproto/scion/go/lib/addr"
)

type scionAddress struct {
	ia libaddr.IA
	l3 libaddr.HostAddr
}

var (
	hostFilePath = "/etc/hosts"
	addrRegexp   = regexp.MustCompile(`^(?P<ia>\d+-[\d:A-Fa-f]+),\[(?P<host>[^\]]+)\]`)
	hosts        map[string]scionAddress // hostname -> scionAddress
	revHosts     map[string][]string     // SCION address w/o port -> hostnames
)

const (
	iaIndex = iota + 1
	l3Index
)

func init() {
	hostsFile, err := readHostsFile()
	if err != nil {
		hostsFile = []byte{}
	}
	parseHostsFile(hostsFile)
	if err != nil {
		log.Fatal(err)
	}
}

// AddHost adds a host to the map of known hosts
// An error is returned if the address has a wrong format or
// the hostname already exists
func AddHost(hostname, address string) error {
	if addrs, ok := hosts[hostname]; ok {
		return fmt.Errorf("Host %q already exists, address(es): %v", hostname, addrs)
	}
	addr, err := addrFromString(address)
	if err != nil {
		return fmt.Errorf("Cannot add host %q: %v", hostname, err)
	}
	hosts[hostname] = addr
	revHosts[address] = append(revHosts[address], hostname)

	return nil
}

// GetHostByName returns the IA and HostAddr corresponding to hostname
func GetHostByName(hostname string) (libaddr.IA, libaddr.HostAddr, error) {
	addr, ok := hosts[hostname]
	if !ok {
		return libaddr.IA{}, nil, fmt.Errorf("Address for host %q not found", hostname)
	}
	return addr.ia, addr.l3, nil
}

// GetHostnamesByAddress returns the hostnames corresponding to address
func GetHostnamesByAddress(address string) ([]string, error) {
	match := addrRegexp.FindString(address)
	host, ok := revHosts[match]
	if !ok {
		return []string{}, fmt.Errorf("Hostname for address %q not found", address)
	}
	return host, nil
}

func readHostsFile() ([]byte, error) {
	bs, err := ioutil.ReadFile(hostFilePath)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func parseHostsFile(hostsFile []byte) {
	hosts = make(map[string]scionAddress)
	revHosts = make(map[string][]string)
	lines := bytes.Split(hostsFile, []byte("\n"))
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			continue
		}
		if matched := addrRegexp.MatchString(fields[0]); matched {
			addr, err := addrFromString(fields[0])
			if err != nil {
				continue
			}

			// map hostnames to scionAddress
			for _, field := range fields[1:] {
				if _, ok := hosts[field]; !ok {
					hosts[field] = addr
					revHosts[fields[0]] = append(revHosts[fields[0]], field)
				}
			}
		}

	}
}

func addrFromString(addr string) (scionAddress, error) {
	parts := addrRegexp.FindStringSubmatch(addr)
	ia, err := libaddr.IAFromString(parts[iaIndex])
	if err != nil {
		return scionAddress{}, fmt.Errorf("Invalid IA string: %v", parts[iaIndex])
	}
	var l3 libaddr.HostAddr
	if hostSVC := libaddr.HostSVCFromString(parts[l3Index]); hostSVC != libaddr.SvcNone {
		l3 = hostSVC
	} else {
		l3 = libaddr.HostFromIPStr(parts[l3Index])
		if l3 == nil {
			return scionAddress{}, fmt.Errorf("Invalid IP address string: %v", parts[l3Index])
		}
	}
	return scionAddress{ia, l3}, nil
}
