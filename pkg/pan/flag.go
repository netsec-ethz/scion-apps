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
	"net/netip"
	"strconv"
)

// IPPortValue implements the flag.Value for a net/netip.AddrPort,
// using the ParseAddrPort function.
type IPPortValue netip.AddrPort

func (v *IPPortValue) Get() netip.AddrPort {
	return netip.AddrPort(*v)
}

func (v *IPPortValue) Set(s string) error {
	val, err := ParseOptionalIPPort(s)
	*v = IPPortValue(val)
	return err
}

func (v *IPPortValue) String() string {
	return netip.AddrPort(*v).String()
}

// ParseOptionalIPPort parses a string to netip.AddrPort
// This accepts either of the following formats
//
//   - <ip>:<port>
//   - :<port>
//   - (empty)
//
// This is provided by this package as typical usage of the Dial/Listen
// will allow to provide the local address as a string, where the omitting
// the IP is a convenient shortcut, valid for both IPv4 and IPv6.
func ParseOptionalIPPort(s string) (netip.AddrPort, error) {
	if s == "" {
		return netip.AddrPort{}, nil
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("unable to parse IP:Port (%q): %w", s, err)
	}
	port16, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid port %q parsing %q", port, s)
	}
	var ip netip.Addr
	if host != "" {
		ip, err = netip.ParseAddr(host)
		if err != nil {
			return netip.AddrPort{}, err
		}
	}
	return netip.AddrPortFrom(ip, uint16(port16)), nil
}
