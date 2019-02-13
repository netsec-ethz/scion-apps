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
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	hostFilePath = "/etc/hosts"
	addrRegexp   = regexp.MustCompile(`^(?P<ia>\d+-[\d:A-Fa-f]+),\[(?P<host>[^\]]+)\]`)
	hosts        map[string]string   // hostname -> SCION address
	revHosts     map[string][]string // SCION address w/o port -> hostnames
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
	err = parseHostsFile(hostsFile)
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
	_, err := snet.AddrFromString(fmt.Sprintf("%s:%s", address, "0"))
	if err != nil {
		return fmt.Errorf("Cannot add host %q: %v", hostname, err)
	}
	hosts[hostname] = address
	revHosts[address] = append(revHosts[address], hostname)

	return nil
}

// GetHostByName returns the IA and HostAddr corresponding to hostname
func GetHostByName(hostname string) (ia libaddr.IA, l3 libaddr.HostAddr, err error) {
	addr, ok := hosts[hostname]
	if !ok {
		err = fmt.Errorf("Address for host %q not found", hostname)
		return
	}

	parts := addrRegexp.FindStringSubmatch(addr)
	ia, err = libaddr.IAFromString(parts[iaIndex])
	if err != nil {
		err = fmt.Errorf("Invalid IA string: %v", parts[iaIndex])
		return
	}
	if hostSVC := libaddr.HostSVCFromString(parts[l3Index]); hostSVC != libaddr.SvcNone {
		l3 = hostSVC
	} else {
		l3 = libaddr.HostFromIPStr(parts[l3Index])
		if l3 == nil {
			err = fmt.Errorf("Invalid IP address string: %v", parts[l3Index])
			return
		}
	}
	return

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

func parseHostsFile(hostsFile []byte) error {
	hosts = make(map[string]string)
	revHosts = make(map[string][]string)
	lines := bytes.Split(hostsFile, []byte("\n"))
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			continue
		}
		if matched := addrRegexp.MatchString(fields[0]); matched {
			for _, field := range fields[1:] {
				if _, ok := hosts[field]; !ok {
					hosts[field] = fields[0]
					revHosts[fields[0]] = append(revHosts[fields[0]], field)
				}
			}
		}

	}
	return nil
}
