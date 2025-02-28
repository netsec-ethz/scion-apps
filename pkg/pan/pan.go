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

# Policy

A path policy defines the allowed paths and/or a preference order of the paths.
Policies are generally stateless and, in particular, they don't look for any
short term information like measured latency or path "liveness".
Connections allow to change the path policy at any time.

# Selector

A path selector is a stateful controller associated with a connection/socket.
It receives the paths filtered by the Policy as an input. For each packet
sent, the selector chooses the path.
The default selector keeps using the first chosen path unless SCMP path down
notifications are encountered, in which case it will always switch to the next
alive path.
Custom selectors implement e.g. active path probing, coupling of multiple
connections to either use the same path or to use maximally disjoint paths,
direct performance feedback from the application, etc.

# Dialed vs Listening

pan differentiates between dialed and listening sockets. Dialed sockets (for
"clients") define the path policy and a selector. The client side of a connection
is in control of the path used.
The listening side (for "servers") only replies on the paths last used by each
client, by means of a customizable reply path selector. The listening side does
not implement any policy, nor does it do anything to keep the paths fresh.
The default reply path selector records a fixed number of paths used by a client.
It normally uses the path last used by the client for replies, but does use other
recorded paths to try routing around temporarily broken paths.

# SCION daemon connection

The SCION daemon is assumed to be at the default address, but this can be
overridden using an environment variable:

	SCION_DAEMON_ADDRESS: 127.0.0.1:30255

This is convenient for the normal use case of running the endhost stack for a
single SCION AS. When running multiple local ASes, e.g. during development, the
address of the SCION daemon corresponding to the desired AS needs to be
specified in the SCION_DAEMON_ADDRESS environment variable.

# Wildcard IP Addresses

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

import (
	"context"
	"fmt"
)

// ResolveUDPAddr parses the address and resolves the hostname.
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of "hostname:port".
// If the address is in the form of a hostname, the the following sources will
// be used to resolve a name, in the given order of precedence.
//
//   - /etc/hosts
//   - /etc/scion/hosts
//   - RAINS, if a server is configured in /etc/scion/rains.cfg. Disabled if built with !norains.
//   - DNS TXT records using the local DNS resolver (depending on OS config, see "Name Resolution" in net package docs)
//
// Returns HostNotFoundError if none of the sources did resolve the hostname.
func ResolveUDPAddr(ctx context.Context, address string) (UDPAddr, error) {
	return resolveUDPAddrAt(ctx, address, defaultResolver())
}

// HostNotFoundError is returned by ResolveUDPAddr when the name was not found, but
// otherwise no error occurred.
type HostNotFoundError struct {
	Host string
}

func (e HostNotFoundError) Error() string {
	return fmt.Sprintf("host not found: '%s'", e.Host)
}
