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
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

// hosts file
var (
	hostFilePath = "/etc/hosts"
	addrRegexp   = regexp.MustCompile(`^(?P<ia>\d+-[\d:A-Fa-f]+),\[(?P<host>[^\]]+)\]`)
	hosts        = make(map[string]snet.SCIONAddress) // hostname -> scionAddress
	revHosts     = make(map[string][]string)          // SCION address w/o port -> hostnames
)

// RAINS
var (
	rainsConfigPath = "/etc/scion/rains.cfg"
	ctx             = "."                    // use global context
	qType           = rains.OTScionAddr4     // request SCION IPv4 addresses
	qOpts           = []rains.Option{}       // no options
	expire          = 5 * time.Minute        // sensible expiry date?
	timeout         = 500 * time.Millisecond // timeout for query
	rainsServer     *snet.Addr               // resolver address
)

const (
	iaIndex = iota + 1
	l3Index
)

func init() {
	// parse hosts file
	hostsFile, err := readHostsFile()
	if err == nil {
		parseHostsFile(hostsFile)
	}

	// read RAINS server address
	rainsServer = readRainsConfig()
}

// AddHost adds a host to the map of known hosts
// An error is returned if the address has a wrong format or
// the hostname already exists
// The added host will not persist between program executions
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

func ResolveUDPAddr(address string) (*snet.Addr, error) {
	raddr, err := snet.AddrFromString(address)
	if err == nil {
		return raddr, nil
	}
	hostStr, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	host, err := GetHostByName(hostStr)
	if err != nil {
		return nil, err
	}
	ia := host.IA
	udp := addr.AppAddrFromUDP(&net.UDPAddr{IP: host.Host.IP(), Port: port})
	return &snet.Addr{IA: ia, Host: udp}, nil
}

// GetHostByName returns the IA and HostAddr corresponding to hostname
func GetHostByName(hostname string) (snet.SCIONAddress, error) {
	// try to resolve hostname locally
	addr, ok := hosts[hostname]
	if ok {
		return addr, nil
	}

	if rainsServer == nil {
		return snet.SCIONAddress{}, fmt.Errorf("Could not resolve %q, no RAINS server configured", hostname)
	}

	// fall back to RAINS

	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The timeout value has been decreased to counter this behavior until the problem is resolved.
	reply, err := rains.Query(hostname, ctx, []rains.Type{qType}, qOpts, expire, timeout, rainsServer)
	if err != nil {
		return snet.SCIONAddress{}, fmt.Errorf("Address for host %q not found: %v", hostname, err)
	}
	scionAddr, err := addrFromString(reply[qType])
	if err != nil {
		return snet.SCIONAddress{}, fmt.Errorf("Address for host %q invalid: %v", hostname, err)
	}

	return scionAddr, nil
}

// GetHostnamesByAddress returns the hostnames corresponding to address
// TODO: (chaehni) RAINS address query to resolve address to name
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
	lines := bytes.Split(hostsFile, []byte("\n"))
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			continue
		}
		if matched := addrRegexp.MatchString(fields[0]); matched {
			address, err := addrFromString(fields[0])
			if err != nil {
				continue
			}

			// map hostnames to scionAddress
			for _, field := range fields[1:] {
				if _, ok := hosts[field]; !ok {
					hosts[field] = address
					revHosts[fields[0]] = append(revHosts[fields[0]], field)
				}
			}
		}
	}
}

func readRainsConfig() *snet.Addr {
	bs, err := ioutil.ReadFile(rainsConfigPath)
	if err != nil {
		return nil
	}
	address, err := snet.AddrFromString(strings.TrimSpace(string(bs)))
	if err != nil {
		return nil
	}
	return address
}

func addrFromString(address string) (snet.SCIONAddress, error) {
	parts := addrRegexp.FindStringSubmatch(address)
	if parts == nil {
		return snet.SCIONAddress{}, fmt.Errorf("No valid SCION address: %q", address)
	}
	ia, err := addr.IAFromString(parts[iaIndex])
	if err != nil {
		return snet.SCIONAddress{}, fmt.Errorf("Invalid IA string: %v", parts[iaIndex])
	}
	var l3 addr.HostAddr
	if hostSVC := addr.HostSVCFromString(parts[l3Index]); hostSVC != addr.SvcNone {
		l3 = hostSVC
	} else {
		l3 = addr.HostFromIPStr(parts[l3Index])
		if l3 == nil {
			return snet.SCIONAddress{}, fmt.Errorf("Invalid IP address string: %v", parts[l3Index])
		}
	}
	return snet.SCIONAddress{IA: ia, Host: l3}, nil
}
