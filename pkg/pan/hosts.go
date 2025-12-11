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
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
)

var (
	resolveEtcHosts      Resolver = &hostsfileResolver{"/etc/hosts"}
	resolveEtcScionHosts Resolver = &hostsfileResolver{"/etc/scion/hosts"}
	resolveRains         Resolver = nil
	resolveDNSTxt        Resolver = &dnsResolver{net.DefaultResolver}
)

// ResolveUDPAddrAt parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, resolver is used to resolve the name.
func ResolveUDPAddrAt(ctx context.Context, address string, resolver Resolver) (UDPAddr, error) {
	raddr, err := ParseUDPAddr(address)
	if err == nil {
		return raddr, nil
	}
	hostStr, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return UDPAddr{}, fmt.Errorf("invalid address: %w", err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return UDPAddr{}, fmt.Errorf("invalid port: %w", err)
	}
	host, err := resolver.Resolve(ctx, hostStr)
	if err != nil {
		return UDPAddr{}, fmt.Errorf("using resolver: %w", err)
	}
	return host.WithPort(uint16(port)), nil
}

// defaultResolver returns the default name resolver, used in ResolveUDPAddr.
// It will use the following sources, in the given order of precedence, to
// resolve a name:
//
//   - /etc/hosts
//   - /etc/scion/hosts
//   - RAINS, if a server is configured in /etc/scion/rains.cfg. Disabled if built with !norains.
//   - DNS TXT records using the local DNS resolver (depending on OS config, see "Name Resolution" in net package docs)
func defaultResolver() Resolver {
	return resolverList{
		resolveEtcHosts,
		resolveEtcScionHosts,
		resolveRains,
		resolveDNSTxt,
	}
}

// Resolver is the interface to resolve a host name to a SCION host address.
// Currently, this is implemented for reading the system hosts file, a SCION specific hosts file,
// RAINS, and DNS TXT records for SCION of the format "scion=ia,ip"
type Resolver interface {
	// Resolve finds an address for the name.
	// Returns a HostNotFoundError if the name was not found, but otherwise no
	// error occurred.
	Resolve(ctx context.Context, name string) (SCIONAddr, error)
}

// resolverList represents a list of Resolvers that are processed in sequence
// to return the first match.
type resolverList []Resolver

func (resolvers resolverList) Resolve(ctx context.Context, name string) (SCIONAddr, error) {
	var errHostNotFound HostNotFoundError
	var rerr error
	for _, resolver := range resolvers {
		if resolver == nil {
			// skip RAINS resolver when disabled
			continue
		}
		// check ctx to avoid unnecessary calls with already expired context
		if err := ctx.Err(); err != nil {
			rerr = err
			break
		}
		addr, err := resolver.Resolve(ctx, name)
		if err == nil {
			return addr, nil
		} else if !errors.As(err, &errHostNotFound) {
			// do not directly fail on first resolver error
			rerr = err
		}
	}
	if rerr != nil {
		// fmt.Fprintf(os.Stderr, "pan library: resolver error: %w", rerr)
		return SCIONAddr{}, fmt.Errorf("pan library: resolver error: %w", rerr)
	}
	return SCIONAddr{}, HostNotFoundError{name}
}
