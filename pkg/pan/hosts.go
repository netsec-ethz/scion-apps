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
// limitations under the License.

package pan

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/scionproto/scion/go/lib/snet"
)

var (
	resolveEtcHosts      resolver = &hostsfileResolver{"/etc/hosts"}
	resolveEtcScionHosts resolver = &hostsfileResolver{"/etc/scion/hosts"}
	resolveRains         resolver = nil
)

// resolveUDPAddrAt parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, resolver is used to resolve the name.
func resolveUDPAddrAt(address string, resolver resolver) (UDPAddr, error) {
	raddr, err := ParseUDPAddr(address)
	if err == nil {
		return raddr, nil
	}
	hostStr, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return UDPAddr{}, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return UDPAddr{}, err
	}
	host, err := resolver.Resolve(hostStr)
	if err != nil {
		return UDPAddr{}, err
	}
	ia := host.IA
	return UDPAddr{IA: ia, IP: host.IP, Port: port}, nil
}

// defaultResolver returns the default name resolver, used in ResolveUDPAddr.
// It will use the following sources, in the given order of precedence, to
// resolve a name:
//
//  - /etc/hosts
//  - /etc/scion/hosts
//  - RAINS, if a server is configured in /etc/scion/rains.cfg.
//    Disabled if built with !norains.
func defaultResolver() resolver {
	return resolverList{
		resolveEtcHosts,
		resolveEtcScionHosts,
		resolveRains,
	}
}

// scionAddr is a SCION/IP host address.
type scionAddr struct {
	IA IA
	IP net.IP
}

var (
	addrRegexp = regexp.MustCompile(`^(\d+-[\d:A-Fa-f]+),(\[[^\]]+\]|[^\[\]]+)$`)
)

const (
	addrRegexpIaIndex = 1
	addrRegexpL3Index = 2
)

// parseSCIONAddr converts an SCION address string to a SCION address.
func parseSCIONAddr(address string) (scionAddr, error) {
	parts := addrRegexp.FindStringSubmatch(address)
	if parts == nil {
		return scionAddr{}, fmt.Errorf("no valid SCION address: %q", address)
	}
	ia, err := ParseIA(parts[addrRegexpIaIndex])
	if err != nil {
		return scionAddr{},
			fmt.Errorf("invalid IA string: %v", parts[addrRegexpIaIndex])
	}
	l3Trimmed := strings.Trim(parts[addrRegexpL3Index], "[]")
	ip := net.ParseIP(l3Trimmed)
	if ip == nil {
		return scionAddr{},
			fmt.Errorf("invalid IP string: %v", l3Trimmed)
	}
	return scionAddr{IA: ia, IP: ip}, nil
}

func (a scionAddr) String() string {
	return fmt.Sprintf("%s,%s", a.IA, a.IP)
}

// resolver is the interface to resolve a host name to a SCION host address.
// Currently, this is implemented for reading a hosts file and RAINS
type resolver interface {
	// Resolve finds an address for the name.
	// Returns a HostNotFoundError if the name was not found, but otherwise no
	// error occurred.
	Resolve(name string) (scionAddr, error)
}

// resolverList represents a list of Resolvers that are processed in sequence
// to return the first match.
type resolverList []resolver

func (resolvers resolverList) Resolve(name string) (scionAddr, error) {
	var errHostNotFound HostNotFoundError
	for _, resolver := range resolvers {
		if resolver != nil {
			addr, err := resolver.Resolve(name)
			if err == nil {
				return addr, nil
			} else if !errors.As(err, &errHostNotFound) {
				return addr, err
			}
		}
	}
	return scionAddr{}, HostNotFoundError{name}
}

var (
	hostPortRegexp = regexp.MustCompile(`^((?:[-.\da-zA-Z]+)|(?:\d+-[\d:A-Fa-f]+,(?:\[[^\]]+\]|[^\[\]:]+))):(\d+)$`)
)

const (
	hostPortRegexpHostIndex = 1
	hostPortRegexpPortIndex = 2
)

// SplitHostPort splits a host:port string into host and port variables.
// This is analogous to net.SplitHostPort, which however refuses to handle SCION addresses.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
func SplitHostPort(hostport string) (host, port string, err error) {
	match := hostPortRegexp.FindStringSubmatch(hostport)
	if match != nil {
		return match[hostPortRegexpHostIndex], match[hostPortRegexpPortIndex], nil
	}
	return "", "", fmt.Errorf("pan.SplitHostPort: invalid address")
}

// MangleSCIONAddr mangles a SCION address string (if it is one) so it can be
// safely used in the host part of a URL.
func MangleSCIONAddr(address string) string {
	raddr, err := snet.ParseUDPAddr(address)
	if err != nil {
		return address
	}

	// Turn this into [IA,IP]:port format. This is a valid host in a URI, as per
	// the "IP-literal" case in RFC 3986, §3.2.2.
	// Unfortunately, this is not currently compatible with snet.ParseUDPAddr,
	// so this will have to be _unmangled_ before use.
	mangledAddr := fmt.Sprintf("[%s,%s]", raddr.IA, raddr.Host.IP)
	if raddr.Host.Port != 0 {
		mangledAddr += fmt.Sprintf(":%d", raddr.Host.Port)
	}
	return mangledAddr
}

// UnmangleSCIONAddr returns a SCION address that can be parsed with
// with snet.ParseUDPAddr.
// If the input is not a SCION address (e.g. a hostname), the address is
// returned unchanged.
// This parses the address, so that it can safely join host and port, with the
// brackets in the right place. Yes, this means this will be parsed twice.
//
// Assumes that address always has a port (this is enforced by the http3
// roundtripper code)
func UnmangleSCIONAddr(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		panic(fmt.Sprintf("UnmangleSCIONAddr assumes that address is of the form host:port %s", err))
	}
	// brackets are removed from [I-A,IP] part by SplitHostPort, so this can be
	// parsed with ParseUDPAddr:
	udpAddr, err := snet.ParseUDPAddr(host)
	if err != nil {
		return address
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return address
	}
	udpAddr.Host.Port = int(p)
	return udpAddr.String()
}
