// Copyright 2021 ETH Zurich
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
	"fmt"
	"net"
	"strconv"

	"inet.af/netaddr"
)

// IPPortValue implements the flag.Value for a inet.af/netaddr.IPPort,
// using the ParseIPPort function.
// NOTE: once go-1.18 becomes available, we'll switch this package over to
// net/netip.
type IPPortValue netaddr.IPPort

func (v *IPPortValue) Get() netaddr.IPPort {
	return netaddr.IPPort(*v)
}

func (v *IPPortValue) Set(s string) error {
	val, err := ParseOptionalIPPort(s)
	*v = IPPortValue(val)
	return err
}

func (v *IPPortValue) String() string {
	return netaddr.IPPort(*v).String()
}

// ParseOptionalIPPort parses a string to netaddr.IPPort
// This accepts either of the following formats
//
//  - <ip>:<port>
//  - :<port>
//  - (empty)
//
// in contrast to netaddr.ParseOptionalIPPort which disallows omitting the IP.
//
// This is provided by this package as typical usage of the Dial/Listen
// will allow to provide the local address as a string, where the omitting
// the IP is a convenient shortcut, valid for both IPv4 and IPv6.
func ParseOptionalIPPort(s string) (netaddr.IPPort, error) {
	if s == "" {
		return netaddr.IPPort{}, nil
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return netaddr.IPPort{}, fmt.Errorf("unable to parse IP:Port (%q): %w", s, err)
	}
	port16, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return netaddr.IPPort{}, fmt.Errorf("invalid port %q parsing %q", port, s)
	}
	var ip netaddr.IP
	if host != "" {
		ip, err = netaddr.ParseIP(host)
		if err != nil {
			return netaddr.IPPort{}, err
		}
	}
	return netaddr.IPPortFrom(ip, uint16(port16)), nil
}
