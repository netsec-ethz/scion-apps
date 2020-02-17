// Copyright 2020 ETH Zurich
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
Package appnet provides a simplified and functionally extended wrapper interface to the
scionproto/scion package snet.


Dispatcher and SCION daemon sockets

During the hidden initialisation of this package, the dispatcher and sciond sockets are
opened. The sciond connection determines the local IA.
The dispatcher and sciond sockets are assumed to be at default locations, but this can
be overridden using environment variables:

		SCION_DISPATCHER_SOCKET: /run/shm/dispatcher/default.sock
		SCION_DAEMON_SOCKET: /run/shm/sciond/default.sock

This is convenient for the normal use case of running a the endhost stack for
a single SCION AS. When running multiple local ASes, e.g. during development, the path
to the sciond corresponding to the desired AS needs to be specified in the
SCION_DAEMON_SOCKET environment variable.


Wildcard IP Addresses

snet does not currently support binding to wildcard addresses. This will hopefully be
added soon-ish, but in the meantime, this package emulates this functionality.
There is one restriction, that applies to hosts with multiple IP addresses in the AS:
the behaviour will be that of binding to one specific local IP address, which means that
the application will not be reachable using any of the other IP addresses.
Traffic sent will always appear to originate from this specific IP address,
even if that's not the correct route to a destination in the local AS.

This restriction will very likely not cause any issues, as a fairly contrived
network setup would be required. Also, sciond has a similar restriction (binds
to one specific IP address).
*/
package appnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

// Network extends the snet.Network interface by making the local IA and common
// sciond connections public.
// The default singleton instance of this type is obtained by the DefNetwork
// function.
type Network struct {
	snet.Network
	IA            addr.IA
	PathQuerier   snet.PathQuerier
	hostInLocalAS net.IP
}

var defNetwork Network
var initOnce sync.Once

// DefNetwork initialises and returns the singleton default Network.
// Typically, this will not be needed for applications directly, as they can
// use the simplified Dial/Listen functions provided here.
func DefNetwork() *Network {
	initOnce.Do(mustInitDefNetwork)
	return &defNetwork
}

// Dial connects to the address (on the SCION/UDP network).
// The address can be of the form of a SCION address (i.e. of the form "ISD-AS,[IP]:port")
// or in the form of hostname:port.
func Dial(address string) (snet.Conn, error) {
	raddr, err := ResolveUDPAddr(address)
	if err != nil {
		return nil, err
	}
	return DialAddr(raddr)
}

// DialAddr connects to the address (on the SCION/UDP network).
//
// If no path is specified in raddr, DialAddr will choose the first available path.
// This path is never updated during the lifetime of the conn. This does not
// support long lived connections well, as the path *will* expire.
// This is all that snet currently provides, we'll need to add a layer on top
// that updates the paths in case they expire or are revoked.
func DialAddr(raddr *snet.UDPAddr) (snet.Conn, error) {
	if raddr.Path == nil {
		err := SetDefaultPath(raddr)
		if err != nil {
			return nil, err
		}
	}
	laddr := &net.UDPAddr{IP: localIP(raddr)}
	return DefNetwork().Dial(context.TODO(), "udp", laddr, raddr, addr.SvcNone)
}

// Listen acts like net.ListenUDP in a SCION network.
// The listen address or parts of it may be nil or unspecified, signifying to
// listen on a wildcard address.
//
// See note on wildcard addresses in the package documentation.
func Listen(listen *net.UDPAddr) (snet.Conn, error) {
	if listen == nil {
		listen = &net.UDPAddr{}
	}
	if listen.IP == nil || listen.IP.IsUnspecified() {
		listen = &net.UDPAddr{IP: defaultLocalIP(), Port: listen.Port, Zone: listen.Zone}
	}
	return DefNetwork().Listen(context.TODO(), "udp", listen, addr.SvcNone)
}

// ListenPort is a shortcut to Listen on a specific port with a wildcard IP address.
//
// See note on wildcard addresses in the package documentation.
func ListenPort(port uint16) (snet.Conn, error) {
	listen := &net.UDPAddr{IP: defaultLocalIP(), Port: int(port)}
	return DefNetwork().Listen(context.TODO(), "udp", listen, addr.SvcNone)
}

// localAddr returns the source IP address for traffic to raddr. If
// raddr.NextHop is set, it's used to determine the local IP address.
// Otherwise, the default local IP address is returned.
//
// The purpose of this function is to workaround not being able to bind to
// wildcard addresses in snet.
// See note on wildcard addresses in the package documentation.
func localIP(raddr *snet.UDPAddr) net.IP {
	if raddr.NextHop != nil {
		nextHop := raddr.NextHop.IP
		return findSrcIP(nextHop)
	}
	return defaultLocalIP()
}

// defaultLocalIP returns _a_ IP of this host in the local AS.
//
// The purpose of this function is to workaround not being able to bind to
// wildcard addresses in snet.
// See note on wildcard addresses in the package documentation.
func defaultLocalIP() net.IP {
	return findSrcIP(DefNetwork().hostInLocalAS)
}

func mustInitDefNetwork() {
	err := initDefNetwork()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing SCION network: %v\n", err)
		os.Exit(1)
	}
}

func initDefNetwork() error {
	dispatcherPath, err := findDispatcherSocket()
	if err != nil {
		return err
	}
	dispatcher := reliable.NewDispatcher(dispatcherPath)

	sciondPath, err := findSciondSocket()
	if err != nil {
		return err
	}
	sciondConn, err := sciond.NewService(sciondPath).Connect(context.Background())
	if err != nil {
		return err
	}
	localIA, err := findLocalIA(sciondConn)
	if err != nil {
		return err
	}
	hostInLocalAS, err := findAnyHostInLocalAS(sciondConn)
	if err != nil {
		return err
	}
	pathQuerier := sciond.Querier{Connector: sciondConn, IA: localIA}
	n := snet.NewNetworkWithPR(
		localIA,
		dispatcher,
		pathQuerier,
		sciond.RevHandler{Connector: sciondConn},
	)
	defNetwork = Network{Network: n, IA: localIA, PathQuerier: pathQuerier, hostInLocalAS: hostInLocalAS}
	return nil
}

func findSciondSocket() (string, error) {
	path, ok := os.LookupEnv("SCION_DAEMON_SOCKET")
	if !ok {
		path = sciond.DefaultSCIONDPath
	}
	err := statSocket(path)
	if err != nil {
		return "", fmt.Errorf("error looking for SCION daemon socket at %s (override with SCION_DAEMON_SOCKET): %v", path, err)
	}
	return path, nil

}

func findDispatcherSocket() (string, error) {
	path, ok := os.LookupEnv("SCION_DISPATCHER_SOCKET")
	if !ok {
		path = reliable.DefaultDispPath
	}
	err := statSocket(path)
	if err != nil {
		return "", fmt.Errorf("error looking for SCION dispatcher socket at %s (override with SCION_DISPATCHER_SOCKET): %v", path, err)
	}
	return path, nil
}

func statSocket(path string) error {
	fileinfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !isSocket(fileinfo.Mode()) {
		return fmt.Errorf("%s is not a socket (mode: %s)", path, fileinfo.Mode())
	}
	return nil
}

func isSocket(mode os.FileMode) bool {
	return mode&os.ModeSocket != 0
}

func findLocalIA(sciondConn sciond.Connector) (addr.IA, error) {
	asInfo, err := sciondConn.ASInfo(context.TODO(), addr.IA{})
	if err != nil {
		return addr.IA{}, err
	}
	ia := asInfo.Entries[0].RawIsdas.IA()
	return ia, nil
}

// findSrcIP returns the src IP used for traffic destined to dst
func findSrcIP(dst net.IP) net.IP {
	// Use net.Dial to lookup source address. Alternatively, could use netlink.
	udpAddr := net.UDPAddr{IP: dst, Port: 1}
	udpConn, _ := net.DialUDP(udpAddr.Network(), nil, &udpAddr)
	srcIP := udpConn.LocalAddr().(*net.UDPAddr).IP
	udpConn.Close()
	return srcIP
}

// findAnyHostInLocalAS returns the IP address of some (infrastructure) host in the local AS.
func findAnyHostInLocalAS(sciondConn sciond.Connector) (net.IP, error) {
	addr, err := sciond.TopoQuerier{Connector: sciondConn}.OverlayAnycast(context.Background(), addr.SvcBS)
	if err != nil {
		return nil, err
	}
	return addr.IP, nil
}
