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

/*
Package pan provides a policy-based, path aware network library for building
applications supporting SCION natively.


The main entry points for applications are:

 - DialUDP / ListenUDP
 - DialQUIC / ListenQUIC

Both forms of the Dial call allow to specify a Policy and a Selector.


Policy

A path policy defines the allowed paths and/or a preference order of the paths.
Policies are generally stateless and, in particular, they don't look for any
short term information like measured latency or path "liveness".
Connections allow to change the path policy at any time.


Selector

A path selector is a stateful controller associated with a connection/socket.
It receives the paths filtered by the Policy as an input. For each packet
sent, the selector chooses the path.
The default selector keeps using the first chosen path unless SCMP path down
notifications are encountered, in which case it will always switch to the next
alive path.
Custom selectors implement e.g. active path probing, coupling of multiple
connections to either use the same path or to use maximally disjoint paths,
direct performance feedback from the application, etc.


Dialed vs Listening

pan differentiates between dialed and listening sockets. Dialed sockets (for
"clients") define the path policy and a selector. The client side of a connection
is in control of the path used.
The listening side (for "servers") only replies on the paths last used by each
client, by means of a customizable reply path selector. The listening side does
not implement any policy, nor does it do anything to keep the paths fresh.
The default reply path selector records a fixed number of paths used by a client.
It normally uses the path last used by the client for replies, but does use other
recorded paths to try routing around temporarily broken paths.


Dispatcher and SCION daemon connections

During the hidden initialisation of this package, the dispatcher and sciond
connections are opened. The sciond connection determines the local IA.
The dispatcher and sciond sockets are assumed to be at default locations, but this can
be overridden using environment variables:

		SCION_DISPATCHER_SOCKET: /run/shm/dispatcher/default.sock
		SCION_DAEMON_ADDRESS: 127.0.0.1:30255

This is convenient for the normal use case of running the endhost stack for a
single SCION AS. When running multiple local ASes, e.g. during development, the
address of the sciond corresponding to the desired AS needs to be specified in
the SCION_DAEMON_ADDRESS environment variable.


Wildcard IP Addresses

The SCION end host stack does not currently support binding to wildcard addresses.
This will hopefully be added eventually, but in the meantime this package resolves
wildcard addresses to a default local IP address when creating a socket.
Binding to one specific local IP address, means that the application will not be reachable at any of
the other IP addresses of the host. Traffic sent will always appear to originate from this specific
IP address, even if that's not the correct route to a destination in the local AS.


Notes

 - pan only performs path lookups for destinations requested by the application.
   Path lookup for an unverified peer could easily be abused for various attacks.
 - Recording the reply path for each peer can be vulnerable to source address spoofing. This
   can potentially be abused to hijack connections.
	 The plan is to require source authentication.
 - In order to allow more explicit control over paths for the listening side,
   plan is to add an explicit "Dial" function to the ListenerConn. There are a
   few different options for this, and none is particularly great (either
   awkward API or performance overhead), so deferred until requirements become clearer.
 - To allow isolation of different application "contexts" that need to avoid leaking path usage
	 information, the plan is to encapsulating the global state of this package in a single object
	 that can be overridden in the context.Context passed to Dial/Listen.
*/
package pan

/*
Project Meta

Goals:
 - make it easy to write *correct* applications using SCION
 - support all common use cases, usable for all our demo applications and
 - replace appnet/appquic
 - self-contained, avoid that user needs to import additional libraries (like snet etc)

Non-goals:
 - expose all low level details or allow to tweak every parameter...
 - replace snet for low-level stuff

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
 - policy/selector as the main concept that applications use to choose paths for
   a connection. Allows the library to implement path updates, fallback,
   performance optimisations, etc. behind the scenes.

Features / Usecases:
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
   -> this is interesting e.g. for "grid-FTP" style application with multiple QUIC sessions

 - http/quic with path control
   - application can give policy
   - application can change policy
   - application can somehow determine currently used path (ok if this is not part of "stable" API)
   - application can change currently used path
   - in a UI like some browser extension, a user may e.g.
     - see the currently used path, see the dial race and path failover
     - explicitly forbid/unforbid a specific path, switch away if it's the current path
     - force use of a specific path

TODO limit resources used by listener; max sessions & cleanup after timeout, also for stats
TODO explicit cleanup of listener conn paths when quic session ends
TODO pick name. Other ideas: supa sap scope, ship: helm, rudder, pilot, scout, spy, till, tiller, skipper
*/

import (
	"fmt"
)

// ResolveUDPAddr parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, the the following sources will
// be used to resolve a name, in the given order of precedence.
//
//  - /etc/hosts
//  - /etc/scion/hosts
//  - RAINS, if a server is configured in /etc/scion/rains.cfg.
//    Disabled if built with !norains.
//
// Returns HostNotFoundError if none of the sources did resolve the hostname.
func ResolveUDPAddr(address string) (UDPAddr, error) {
	return resolveUDPAddrAt(address, defaultResolver())
}

// HostNotFoundError is returned by ResolveUDPAddr when the name was not found, but
// otherwise no error occurred.
type HostNotFoundError struct {
	Host string
}

func (e HostNotFoundError) Error() string {
	return fmt.Sprintf("host not found: '%s'", e.Host)
}
