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
	"regexp"
	"strconv"
	"strings"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/snet"
	"inet.af/netaddr"
)

// UDPAddr is an address for a SCION/UDP end point.
type UDPAddr struct {
	IA   IA
	IP   netaddr.IP
	Port uint16
}

func (a UDPAddr) Network() string {
	return "scion+udp"
}

func (a UDPAddr) String() string {
	if a.IP.Is6() {
		return fmt.Sprintf("%s,[%s]:%d", a.IA, a.IP, a.Port)
	} else {
		return fmt.Sprintf("%s,%s:%d", a.IA, a.IP, a.Port)
	}
}

// IsZero reports whether a is the zero value of the UDPAddr type.
func (a UDPAddr) IsZero() bool {
	return a == UDPAddr{}
}

// IsValue reports whether a is a valid address; IA is not a wildcard (and thus
// not zero) and IP is initialized (may be "0.0.0.0" or "::"). All ports are
// valid, including zero.
func (a UDPAddr) IsValid() bool {
	return !a.IA.IsWildcard() && a.IP.IsValid()
}

func (a UDPAddr) WithPort(port uint16) UDPAddr {
	return UDPAddr{IA: a.IA, IP: a.IP, Port: port}
}

func (a UDPAddr) scionAddr() scionAddr {
	return scionAddr{IA: a.IA, IP: a.IP}
}

func (a UDPAddr) snetUDPAddr() *snet.UDPAddr {
	return &snet.UDPAddr{
		IA:   addr.IA(a.IA),
		Host: netaddr.IPPortFrom(a.IP, a.Port).UDPAddr(),
	}
}

// Set implements flag.Value
func (a *UDPAddr) Set(s string) error {
	var err error
	*a, err = ParseUDPAddr(s)
	return err
}

// ParseUDPAddr converts an address string to a SCION address.
func ParseUDPAddr(s string) (UDPAddr, error) {
	addr, err := snet.ParseUDPAddr(s)
	if err != nil {
		return UDPAddr{}, err
	}
	ip, ok := netaddr.FromStdIP(addr.Host.IP)
	if !ok {
		panic("snet.ParseUDPAddr returned invalid IP")
	}
	return UDPAddr{
		IA:   IA(addr.IA),
		IP:   ip,
		Port: uint16(addr.Host.Port),
	}, nil
}

// MustParseUDPAddr calls ParseUDPAddr and panics on error. This is
// intended for testing.
func MustParseUDPAddr(s string) UDPAddr {
	addr, err := ParseUDPAddr(s)
	if err != nil {
		panic(err)
	}
	return addr
}

type IA addr.IA

// IsZero reports whether ia is the zero value of the IA type.
func (ia IA) IsZero() bool {
	return ia == 0
}

// IsWildcard reports whether ia has a wildcard part (isd or as, or both).
func (ia IA) IsWildcard() bool {
	return addr.IA(ia).IsWildcard()
}

func (ia IA) String() string {
	return addr.IA(ia).String()
}

// ParseIA parses an IA from a string of the format 'ia-as'.
func ParseIA(s string) (IA, error) {
	ia, err := addr.ParseIA(s)
	return IA(ia), err
}

// MustParseIA calls ParseIA and panics on error. This is
// intended for testing.
func MustParseIA(s string) IA {
	ia, err := ParseIA(s)
	if err != nil {
		panic(err)
	}
	return ia
}

// scionAddr is a SCION/IP host address.
// Not exported for now as it's not used in the API for now. Might be
// useful for applicications later.
type scionAddr struct {
	IA IA
	// TODO(JordiSubira): decide where and how to convert to netip from
	// inet.af...
	IP netaddr.IP
}

func (a scionAddr) String() string {
	return fmt.Sprintf("%s,%s", a.IA, a.IP)
}

func (a scionAddr) WithPort(port uint16) UDPAddr {
	return UDPAddr{IA: a.IA, IP: a.IP, Port: port}
}

func (a scionAddr) snetUDPAddr() *snet.UDPAddr {
	return &snet.UDPAddr{
		IA:   addr.IA(a.IA),
		Host: netaddr.IPPortFrom(a.IP, 0).UDPAddr(),
	}
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
		return scionAddr{}, parseSCIONAddrError{in: address, msg: "unable to parse SCION address"}
	}
	ia, err := ParseIA(parts[addrRegexpIaIndex])
	if err != nil {
		return scionAddr{}, parseSCIONAddrError{in: address, msg: "invalid IA", cause: err}
	}
	l3Trimmed := strings.Trim(parts[addrRegexpL3Index], "[]")
	ip, err := netaddr.ParseIP(l3Trimmed)
	if err != nil {
		return scionAddr{}, parseSCIONAddrError{in: address, msg: "invalid IP", cause: err}
	}
	return scionAddr{IA: ia, IP: ip}, nil
}

type parseSCIONAddrError struct {
	in    string // the string given to parseSCIONAddr
	msg   string // an explanation of the parse failure
	cause error  // wrapped error
}

func (err parseSCIONAddrError) Error() string {
	if err.cause != nil {
		return fmt.Sprintf("ParseSCIONAddr(%q): %s, %v:", err.in, err.msg, err.cause)
	}
	return fmt.Sprintf("ParseSCIONAddr(%q): %s", err.in, err.msg)
}

func (err parseSCIONAddrError) Unwrap() error {
	return err.cause
}

// mustParseSCIONAddr calls parseSCIONAddr and panics on error.
func mustParseSCIONAddr(s string) scionAddr {
	addr, err := parseSCIONAddr(s)
	if err != nil {
		panic(err)
	}
	return addr
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
	return "", "", fmt.Errorf("pan.SplitHostPort: invalid address (%q)", hostport)
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
