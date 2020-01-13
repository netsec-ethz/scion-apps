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
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

var addrRegexp = regexp.MustCompile(`^(\d+-[\d:A-Fa-f]+),\[([^\]]+)\]$`)
var hostPortRegexp = regexp.MustCompile(`^((?:[-.\da-zA-Z]+)|(?:\d+-[\d:A-Fa-f]+,\[[^\]]+\])):(\d+)$`)

// hosts file
const hostFilePath = "/etc/hosts"

type hostsTable struct {
	byName map[string]snet.SCIONAddress // hostname -> scionAddress
	byAddr map[string][]string          // SCION address (w/o port) -> hostnames
}

func newHostsTable() hostsTable {
	return hostsTable{
		byName: make(map[string]snet.SCIONAddress),
		byAddr: make(map[string][]string),
	}
}

func (h hostsTable) add(name string, addr snet.SCIONAddress) bool {
	if _, ok := h.byName[name]; !ok {
		h.byName[name] = addr
		addrStr := addrToString(addr)
		h.byAddr[addrStr] = append(h.byAddr[addrStr], name)
		return true
	}
	return false
}

var loadHostsOnce sync.Once
var hostsTableInstance hostsTable

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
	// read RAINS server address
	rainsServer = readRainsConfig()
}

// SplitHostPort splits a host:port string into host and port variables.
// This is analogous to net.SplitHostPort, which however refuses to handle SCION addresses.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
func SplitHostPort(hostport string) (host, port string, err error) {
	match := hostPortRegexp.FindStringSubmatch(hostport)
	if match != nil {
		return match[1], match[2], nil
	}
	return "", "", fmt.Errorf("appnet.SplitHostPort: invalid address")
}

// ResolveUDPAddr parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
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
	if err != nil {
		return nil, err
	}
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
	addr, ok := hosts().byName[hostname]
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

// AddHost adds a host to the map of known hosts
// An error is returned if the address has a wrong format or
// the hostname already exists
// The added host will not persist between program executions
func AddHost(hostname, address string) error {
	addr, err := addrFromString(address)
	if err != nil {
		return fmt.Errorf("Cannot add host %q: %v", hostname, err)
	}
	if !hosts().add(hostname, addr) {
		return fmt.Errorf("Host %q already exists", hostname)
	}

	return nil
}

// GetHostnamesByAddress returns the hostnames corresponding to address
// TODO: (chaehni) RAINS address query to resolve address to name
func GetHostnamesByAddress(address snet.SCIONAddress) ([]string, error) {

	host, ok := hosts().byAddr[addrToString(address)]
	if !ok {
		return []string{}, fmt.Errorf("Hostname for address %q not found", address)
	}
	return host, nil
}

func hosts() *hostsTable {
	loadHostsOnce.Do(func() {
		hostsTableInstance = loadHostsFile(hostFilePath)
	})
	return &hostsTableInstance
}

func loadHostsFile(path string) hostsTable {
	hostsFile, err := readHostsFile(path)
	if err == nil {
		return parseHostsFile(hostsFile)
	}
	return newHostsTable()
}

func readHostsFile(path string) ([]byte, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func parseHostsFile(hostsFile []byte) hostsTable {
	hosts := newHostsTable()
	lines := bytes.Split(hostsFile, []byte("\n"))
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			continue
		}
		if addrRegexp.MatchString(fields[0]) {
			addr, err := addrFromString(fields[0])
			if err != nil {
				continue
			}

			// map hostnames to scionAddress
			for _, field := range fields[1:] {
				_ = hosts.add(field, addr)
			}
		}
	}
	return hosts
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

// addrFromString parses a string to a snet.SCIONAddress
// XXX(matzf) this would optimally be part of snet
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

// addrToString formats an snet.SCIONAddress as a string
// XXX(matzf) this would optimally be part of snet
func addrToString(addr snet.SCIONAddress) string {
	return fmt.Sprintf("%s,[%s]", addr.IA, addr.Host)
}
