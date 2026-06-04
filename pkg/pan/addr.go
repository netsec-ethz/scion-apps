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
	"regexp"
	"strconv"
	"strings"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/connect"
	"github.com/scionproto/scion/pkg/snet"
)

// UDPAddr is an address for a SCION/UDP end point.
type UDPAddr struct {
	IA   IA
	IP   netip.Addr
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

// Set implements flag.Value
func (a *UDPAddr) Set(s string) error {
	var err error
	*a, err = ParseUDPAddr(s)
	return err
}

func UDPAddrFromSnetUdp(s *snet.UDPAddr) UDPAddr {
	ip, ok := netip.AddrFromSlice(s.Host.IP)
	if !ok {
		panic("snet.ParseUDPAddr returned invalid IP")
	}
	return UDPAddr{
		IA:   IA(s.IA),
		IP:   ip.Unmap(),
		Port: uint16(s.Host.Port),
	}
}

// ParseUDPAddr converts an address string to a SCION address.
func ParseUDPAddr(s string) (UDPAddr, error) {
	addr, err := snet.ParseUDPAddr(s)
	if err != nil {
		return UDPAddr{}, err
	}
	return UDPAddrFromSnetUdp(addr), nil
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
	IP netip.Addr
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
		Host: net.UDPAddrFromAddrPort(netip.AddrPortFrom(a.IP, 0)),
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
	ip, err := netip.ParseAddr(l3Trimmed)
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
// safely used in the host part of a URL. It relies on the scionproto's connect.Baseurl function.
// Examples:
//   - "1-ff00:0:110,10.0.0.1:31000" -> "scion4-1-ff00-0-110_10-0-0-1:31000"
//   - "1-ff00:0:110,[::1]:31000"    -> "scion6-1-ff00-0-110_--1:31000"
//   - "1-ff00:0:110,CS"             -> "scion-1-ff00-0-110_CS"
func MangleSCIONAddr(address string) string {
	var a net.Addr
	if raddr, err := snet.ParseUDPAddr(address); err == nil {
		// A scion UDP address.
		a = raddr
	} else if svcAddr, err := parseSvcAddr(address); err == nil {
		// A scion SVC address.
		a = svcAddr
	} else {
		// Anything else.
		return address
	}

	// Rely on BaseUrl to mangle the address.
	mangled := connect.BaseUrl(a)[8:] // remove the "https://" part.
	// BaseUrl always returns the form https://mangled-host-address:port.
	parts := strings.Split(mangled, ":")
	if len(parts) != 2 {
		// This should never happen.
		return mangled
	}
	if parts[1] == "0" {
		// Remove the not-provided port.
		mangled = mangled[:len(mangled)-2]
	}

	return mangled
}

var mangledScionHostPort = regexp.MustCompile(`^scion(\d?)-(.+)$`)

// UnmangleSCIONAddr turns a scion-mangled URL like scion4-1-ff00-0-110_10-0-0-1:31000 into
// a valid snet.UDPAddr like that from 1-ff00:0:110,10.0.0.1:31000.
// This function is necessary if any dialer receives a mangled host-port, such as in the
// Dialer.DialContext function of pkg/shttp
func UnmangleSCIONAddr(mangled string) string {
	match := mangledScionHostPort.FindStringSubmatch(mangled)
	if len(match) != 3 {
		// This is not a scion-mangled host:port.
		return mangled
	}
	ipType := match[1]
	encodedAddr := match[2]

	scionAddr := snet.UDPAddr{}
	// Replace _ to , in the mangled address.
	encodedAddr = strings.ReplaceAll(encodedAddr, "_", ",")
	// Split the inter and intra AS addresses.
	parts := strings.Split(encodedAddr, ",")
	if len(parts) != 2 {
		// Not a scion-mangled intra-AS address.
		return mangled
	}

	// Inter-AS part.
	interASAddr := parts[0]
	idx := strings.Index(interASAddr, "-")
	if idx < 0 || idx >= len(interASAddr) {
		// Not an ISD-AS address.
		return mangled
	}
	interASAddr = interASAddr[0:idx+1] + strings.ReplaceAll(interASAddr[idx+1:], "-", ":")
	ia, err := addr.ParseIA(interASAddr)
	if err != nil {
		return mangled
	}
	scionAddr.IA = ia

	// Intra-AS part. Split host and port.
	parts = strings.Split(parts[1], ":")
	if len(parts) == 0 || len(parts) > 2 {
		// Not a host:port.
		return mangled
	}
	host := parts[0]
	port := 0
	if len(parts) == 2 {
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			// Not a numeric port in host:port.
			return mangled
		}
	}

	// Type of intra-AS address:
	switch ipType {
	case "4":
		host = strings.ReplaceAll(host, "-", ".")
	case "6":
		host = strings.ReplaceAll(host, "-", ":")
	default:
		// Not a valid scion-mangled intra-AS address.
		return mangled
	}

	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return mangled
	}
	scionAddr.Host = &net.UDPAddr{
		IP:   ipAddr.IP,
		Port: port,
	}

	return scionAddr.String()
}

// parseSvcAddr parses scion service addresses such as "1-ff00:0:110,CS".
func parseSvcAddr(address string) (*snet.SVCAddr, error) {
	a, err := addr.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	if a.Host.Type() != addr.HostTypeSVC {
		return nil, fmt.Errorf("not a SVC but %s", a.Host.Type())
	}
	return &snet.SVCAddr{
		IA:  a.IA,
		SVC: a.Host.SVC(),
	}, nil
}
