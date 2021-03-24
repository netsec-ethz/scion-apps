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

/* pan, pan ready, pan fried, pandemic, Path Aware Networking, peter pan,

Package pan provides a policy-based path aware network layer for building
applications supporting SCION natively.

Differences to previous approach "pkg/appnet" and scionproto/scion's snet:
- higher level: pan automatically takes care of timely path refresh, fail-over etc.
  The interface to pan talks about path *policies* and path selector
  mechanisms, not individual paths.
- self-contained: does not require importing random internal scion libraries
- address does not contain path. This may seems like a technicality, but having
  the path in the address is really quite awkward when passing addresses through
  layers that do not know about this. For example, in snet/squic, the quic server
  just keeps replying using the path on which the first packet arrived. This
  will seem to work initially, but can't work once this path expires -- working
  around this requires tricksery.

Goals:
- make it easy to write *correct* applications using SCION
- support common use cases

Non-goals:
- expose all low level details or allow to tweak every parameter...


TODO add name resolution, copy-paste from appnet
TODO limit resources used by listener; max sessions & cleanup after timeout, also for stats
TODO explicit cleanup of listener conn paths when quic session ends
TODO hijacking by src address spoofing and bad path.
		 idea: stick to _first_ path initially, asynchronously fetch return paths
		 and allow matching paths (policy!)
*/
package pan

/*
XXX scratchpad
Other name ideas:
  supa
  sap
  scope
  ship:
    helm, rudder, pilot, scout, spy, till, tiller, skipper
    foghorn

Features / Usecases

- select path based on filter and preference
  - filter based on ISD / ASes traversed
  - filter based on attributes (length, latency, ...)
  - order by attributes
  - disjoint from previous paths
- interactive choice
  - optionally with fallback in case path dies
    -> in this mode, manual input can be considered a preference order
- keep paths fresh
  - reevaluate selection or just update selected path?
    -> answer: reevaluate selection; partial order, compare from current
               only update selected should be achievable too (analogous to interactive choice)
- remove dead paths and fail over
  - by SCMP
  - by indication of application
  - by expiration in less than ~10s

- race opening
- path negotiation
- server side path policy?
- active probing
  -> in data stream? out of stream? or only on the side, control?
- active path control from outside (API/user interaction -- see below)

- couple multiple selectors to use different/disjoint paths to maximize bandwidth
  -> correctly handle failover etc.
  -> only need disjointness on bottleneck links (yay for having per link information in metadata!)
  -> this is interesting e.g. for "grid-FTP" style application where multiple QUIC sessions run over

- http/quic with path control
  - application can give policy
  - application can change policy
  - application can somehow determine currently used path (ok if this is not part of "stable" API)
  - application can change currently used path
  - in a UI like some browser extension, a user may e.g.
    - see the currently used path, see the dial race and path failover
    - explicitly forbid/unforbid a specific path, switch away if it's the current path
    - force use of a specific path
*/

import (
	"fmt"
	"net"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

// FIXME: leaking addr.I, addr.A
// TODO: parse IA
type IA addr.IA

func (ia IA) String() string {
	return addr.IA(ia).String()
}

type IfID uint64

// NOTE: does _NOT_ contain path
type UDPAddr struct {
	IA   IA
	IP   net.IP
	Port int
}

func (a UDPAddr) Network() string {
	return "scion+udp"
}

func (a UDPAddr) String() string {
	// TODO: Maybe we can start to experiment with different representations here.
	// I like
	//   isd-as-ipv4:port
	//   [isd-as-ipv6]:port (who cares about zones anyway?)
	if a.IP.To4() == nil {
		return fmt.Sprintf("%s,[%s]:%d", a.IA, a.IP, a.Port)
	} else {
		return fmt.Sprintf("%s,%s:%d", a.IA, a.IP, a.Port)
	}
}

func (a UDPAddr) Equal(x UDPAddr) bool {
	return a.IA == x.IA &&
		a.IP.Equal(x.IP) &&
		a.Port == x.Port
}

func ParseUDPAddr(s string) (UDPAddr, error) {
	addr, err := snet.ParseUDPAddr(s)
	if err != nil {
		return UDPAddr{}, err
	}
	return UDPAddr{
		IA:   IA(addr.IA),
		IP:   addr.Host.IP,
		Port: addr.Host.Port,
	}, nil
}
