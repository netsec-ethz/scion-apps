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

package appnet

import (
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	resolveEtcHosts      Resolver = &hostsfileResolver{"/etc/hosts"}
	resolveEtcScionHosts Resolver = &hostsfileResolver{"/etc/scion/hosts"}
	resolveRains         Resolver = nil
)

var (
	addrRegexp     = regexp.MustCompile(`^(\d+-[\d:A-Fa-f]+),\[([^\]]+)\]$`)
	hostPortRegexp = regexp.MustCompile(`^((?:[-.\da-zA-Z]+)|(?:\d+-[\d:A-Fa-f]+,(\[[^\]]+\]|[^\]:]+))):(\d+)$`)
)

const (
	addrRegexpIaIndex = 1
	addrRegexpL3Index = 2

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
	return "", "", fmt.Errorf("appnet.SplitHostPort: invalid address")
}

// ResolveUDPAddr parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, the DefaultResolver is used to
// resolve the name.
func ResolveUDPAddr(address string) (*snet.UDPAddr, error) {
	return ResolveUDPAddrAt(address, DefaultResolver())
}

// ResolveUDPAddrAt parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, resolver is used to resolve the name.
func ResolveUDPAddrAt(address string, resolver Resolver) (*snet.UDPAddr, error) {

	raddr, err := snet.ParseUDPAddr(address)
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
	host, err := resolver.Resolve(hostStr)
	if err != nil {
		return nil, err
	}
	ia := host.IA
	return &snet.UDPAddr{IA: ia, Host: &net.UDPAddr{IP: host.Host.IP(), Port: port}}, nil
}

// DefaultResolver returns the default name resolver, used in ResolveUDPAddr.
// It will use the following sources, in the given order of precedence, to
// resolve a name:
//
//  - /etc/hosts
//  - /etc/scion/hosts
//  - RAINS, if a server is configured in /etc/scion/rains.cfg.
//    Disabled if built with !norains.
func DefaultResolver() Resolver {
	return ResolverList{
		resolveEtcHosts,
		resolveEtcScionHosts,
		resolveRains,
	}
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

// addrFromString parses a string to a snet.SCIONAddress
// XXX(matzf) this would optimally be part of snet
func addrFromString(address string) (snet.SCIONAddress, error) {

	parts := addrRegexp.FindStringSubmatch(address)
	if parts == nil {
		return snet.SCIONAddress{}, fmt.Errorf("no valid SCION address: %q", address)
	}
	ia, err := addr.IAFromString(parts[addrRegexpIaIndex])
	if err != nil {
		return snet.SCIONAddress{},
			fmt.Errorf("invalid IA string: %v", parts[addrRegexpIaIndex])
	}
	var l3 addr.HostAddr
	if hostSVC := addr.HostSVCFromString(parts[addrRegexpL3Index]); hostSVC != addr.SvcNone {
		l3 = hostSVC
	} else {
		l3 = addr.HostFromIPStr(parts[addrRegexpL3Index])
		if l3 == nil {
			return snet.SCIONAddress{},
				fmt.Errorf("invalid IP address string: %v", parts[addrRegexpL3Index])
		}
	}
	return snet.SCIONAddress{IA: ia, Host: l3}, nil
}
