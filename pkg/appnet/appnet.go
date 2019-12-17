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
// limitations under the License.package main

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

type network struct {
	snet.Network
	localIA       addr.IA
	hostInLocalAS net.IP
	PathQuerier   snet.PathQuerier
}

var defNetwork network
var initOnce sync.Once

// Network returns the singleton snet.Network.
// Typically, this will not be needed for applications directly, as they can
// use the helper Dial/Listen functions provided here.
func Network() *network {
	initOnce.Do(mustInitDefNetwork) // XXX: is it better to init()?
	return &defNetwork
}

func Dial(address string) (snet.Conn, error) {
	raddr, err := ResolveUDPAddr(address)
	if err != nil {
		return nil, err
	}
	return DialAddr(raddr)
}

func DialAddr(raddr *snet.Addr) (snet.Conn, error) {
	laddr := localAddr(raddr)
	return Network().Dial("udp", laddr, ToSNetUDPAddr(raddr), addr.SvcNone, 0)
}

func Listen(listen *net.UDPAddr) (snet.Conn, error) {
	if listen == nil {
		listen = localAddr(nil)
	} else if listen.IP.IsUnspecified() {
		ip := localAddr(nil).IP
		listen = &net.UDPAddr{IP: ip, Port: listen.Port, Zone: listen.Zone}
	}
	return Network().Listen("udp", listen, addr.SvcNone, 0)
}

// ListenPort is a shortcut to listen on a port with a wildcard address
func ListenPort(port uint16) (snet.Conn, error) {
	listen := localAddr(nil)
	listen.Port = int(port)
	return Network().Listen("udp", listen, addr.SvcNone, 0)
}

// localAddr returns _a_ sensible local address. The local IA is determined
// based on the IA of the sciond (which is in turn defined to be either at the
// default path or overriden using SCION_DAEMON_SOCKET).
// If raddr is not nil and NextHop is set, the NextHop will be considered to
// determine the local IP address. Otherwise, the default local IP address is
// used.
// Note: this is only to workaround not being able to bind to wildcard addresses.
// Note: this will NOT work nicely if you expect to e.g. switch between VPN/non-VPN paths.
func localAddr(raddr *snet.Addr) *net.UDPAddr {
	var localIP net.IP
	if raddr != nil && raddr.NextHop != nil {
		nextHop := raddr.NextHop.IP
		localIP = findSrcIP(nextHop)
	} else {
		localIP = findSrcIP(Network().hostInLocalAS)
	}
	return &net.UDPAddr{IP: localIP, Port: 0}
}

// ToSNetUDPAddr is a helper to convert snet.Addr to the newer snet.UDPAddr type
// XXX: snet.Addr will be removed...
func ToSNetUDPAddr(addr *snet.Addr) *snet.UDPAddr {
	return snet.NewUDPAddr(addr.IA, addr.Path, addr.NextHop, &net.UDPAddr{IP: addr.Host.L3.IP(), Port: int(addr.Host.L4)})
}

func mustInitDefNetwork() {
	err := initDefNetwork()
	if err != nil {
		panic(fmt.Errorf("Error initializing SCION network: %v", err))
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
	defNetwork = network{n, localIA, hostInLocalAS, pathQuerier}
	return nil
}

func findSciondSocket() (string, error) {
	path, ok := os.LookupEnv("SCION_DAEMON_SOCKET")
	if !ok {
		path = sciond.DefaultSCIONDPath
	}
	err := statSocket(path)
	if err != nil {
		return "", fmt.Errorf("Error looking for SCION dispatcher socket at %s (override with SCION_DAEMON_SOCKET): %v", path, err)
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
		return "", fmt.Errorf("Error looking for SCION dispatcher socket at %s (override with SCION_DISPATCHER_SOCKET): %v", path, err)
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
