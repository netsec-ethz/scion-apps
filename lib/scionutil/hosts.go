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
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	libaddr "github.com/scionproto/scion/go/lib/addr"
	log "github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
)

type scionAddress struct {
	ia libaddr.IA
	l3 libaddr.HostAddr
}

// hosts file
var (
	hostFilePath = "/etc/hosts"
	addrRegexp   = regexp.MustCompile(`^(?P<ia>\d+-[\d:A-Fa-f]+),\[(?P<host>[^\]]+)\]`)
	hosts        = make(map[string]scionAddress) // hostname -> scionAddress
	revHosts     = make(map[string][]string)     // SCION address w/o port -> hostnames
)

// RAINS
var (
	rainsConfigPath = path.Join(os.Getenv("SC"), "gen", "rains.cfg")
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
	if err != nil {
		hostsFile = []byte{}
	}
	parseHostsFile(hostsFile)
	if err != nil {
		log.Warn("Error parsing hosts file, local name resolution not available", "error", err)
	}

	// read RAINS server address
	srv, err := readRainsConfig()
	if err != nil {
		log.Warn("Could not configure RAINS, remote name resolution not available", "error", err)
	} else {
		rainsServer = srv
	}
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

// GetHostByName returns the IA and HostAddr corresponding to hostname
func GetHostByName(hostname string) (libaddr.IA, libaddr.HostAddr, error) {
	// try to resolve hostname locally
	addr, ok := hosts[hostname]
	if ok {
		return addr.ia, addr.l3, nil
	}

	if rainsServer == nil {
		return libaddr.IA{}, nil, fmt.Errorf("Could not resolve %q, no RAINS server configured", hostname)
	}

	// fall back to RAINS

	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The timeout value has been decreased to counter this behavior until the problem is resolved.
	reply, err := rains.Query(hostname, ctx, []rains.Type{qType}, qOpts, expire, timeout, rainsServer)
	if err != nil {
		return libaddr.IA{}, nil, fmt.Errorf("Address for host %q not found: %v", hostname, err)
	}
	scionAddr, err := addrFromString(reply[qType])
	if err != nil {
		return libaddr.IA{}, nil, fmt.Errorf("Address for host %q invalid: %v", hostname, err)
	}

	return scionAddr.ia, scionAddr.l3, nil

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

func readRainsConfig() (*snet.Addr, error) {
	bs, err := ioutil.ReadFile(rainsConfigPath)
	if err != nil {
		return nil, err
	}
	addr, err := snet.AddrFromString(strings.TrimSpace(string(bs)))
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func addrFromString(addr string) (scionAddress, error) {
	parts := addrRegexp.FindStringSubmatch(addr)
	if parts == nil {
		return scionAddress{}, fmt.Errorf("No valid SCION address: %q", addr)
	}
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
