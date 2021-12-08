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
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/daemon"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/addrutil"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"inet.af/netaddr"
)

// hostContext contains the information needed to connect to the host's local SCION stack,
// i.e. the connection to sciond and dispatcher.
type hostContext struct {
	ia            IA
	sciond        daemon.Connector
	dispatcher    reliable.Dispatcher
	hostInLocalAS net.IP
}

const (
	initTimeout = 1 * time.Second
)

var singletonHostContext hostContext
var initOnce sync.Once

// host initialises and returns the singleton hostContext.
func host() *hostContext {
	initOnce.Do(mustInitHostContext)
	return &singletonHostContext
}

func mustInitHostContext() {
	hostCtx, err := initHostContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing SCION host context: %v\n", err)
		os.Exit(1)
	}
	singletonHostContext = hostCtx
}

func initHostContext() (hostContext, error) {
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()
	dispatcher, err := findDispatcher()
	if err != nil {
		return hostContext{}, err
	}
	sciondConn, err := findSciond(ctx)
	if err != nil {
		return hostContext{}, err
	}
	localIA, err := sciondConn.LocalIA(ctx)
	if err != nil {
		return hostContext{}, err
	}
	hostInLocalAS, err := findAnyHostInLocalAS(ctx, sciondConn)
	if err != nil {
		return hostContext{}, err
	}
	return hostContext{
		ia:            IA(localIA),
		sciond:        sciondConn,
		dispatcher:    dispatcher,
		hostInLocalAS: hostInLocalAS,
	}, nil
}

func findSciond(ctx context.Context) (daemon.Connector, error) {
	address, ok := os.LookupEnv("SCION_DAEMON_ADDRESS")
	if !ok {
		address = daemon.DefaultAPIAddress
	}
	sciondConn, err := daemon.NewService(address).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SCIOND at %s (override with SCION_DAEMON_ADDRESS): %w", address, err)
	}
	return sciondConn, nil
}

func findDispatcher() (reliable.Dispatcher, error) {
	path, err := findDispatcherSocket()
	if err != nil {
		return nil, err
	}
	dispatcher := reliable.NewDispatcher(path)
	return dispatcher, nil
}

func findDispatcherSocket() (string, error) {
	path, ok := os.LookupEnv("SCION_DISPATCHER_SOCKET")
	if !ok {
		path = reliable.DefaultDispPath
	}

	if err := statSocket(path); err != nil {
		return "", fmt.Errorf("error looking for SCION dispatcher socket at %s (override with SCION_DISPATCHER_SOCKET): %w", path, err)
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

// findAnyHostInLocalAS returns the IP address of some (infrastructure) host in the local AS.
func findAnyHostInLocalAS(ctx context.Context, sciondConn daemon.Connector) (net.IP, error) {
	addr, err := daemon.TopoQuerier{Connector: sciondConn}.UnderlayAnycast(ctx, addr.SvcCS)
	if err != nil {
		return nil, err
	}
	return addr.IP, nil
}

// defaultLocalIP returns _a_ IP of this host in the local AS.
//
// The purpose of this function is to workaround not being able to bind to
// wildcard addresses in snet.
// See note on wildcard addresses in the package documentation.
func defaultLocalIP() (netaddr.IP, error) {
	stdIP, err := addrutil.ResolveLocal(host().hostInLocalAS)
	ip, ok := netaddr.FromStdIP(stdIP)
	if err != nil || !ok {
		return netaddr.IP{}, fmt.Errorf("unable to resolve default local address %w", err)
	}
	return ip, nil
}

// defaultLocalAddr fills in a missing or unspecified IP field with defaultLocalIP.
func defaultLocalAddr(local netaddr.IPPort) (netaddr.IPPort, error) {
	if local.IP().IsZero() || local.IP().IsUnspecified() {
		localIP, err := defaultLocalIP()
		if err != nil {
			return netaddr.IPPort{}, err
		}
		local = local.WithIP(localIP)
	}
	return local, nil
}

func (h *hostContext) queryPaths(ctx context.Context, dst IA) ([]*Path, error) {
	flags := daemon.PathReqFlags{Refresh: false, Hidden: false}
	snetPaths, err := h.sciond.Paths(ctx, addr.IA(dst), addr.IA{}, flags)
	if err != nil {
		return nil, err
	}
	paths := make([]*Path, len(snetPaths))
	for i, p := range snetPaths {
		snetMetadata := p.Metadata()
		metadata := &PathMetadata{
			Interfaces:   convertPathInterfaceSlice(snetMetadata.Interfaces),
			MTU:          snetMetadata.MTU,
			Latency:      snetMetadata.Latency,
			Bandwidth:    snetMetadata.Bandwidth,
			Geo:          snetMetadata.Geo,
			LinkType:     snetMetadata.LinkType,
			InternalHops: snetMetadata.InternalHops,
			Notes:        snetMetadata.Notes,
		}
		underlayStd := p.UnderlayNextHop()
		underlay, ok := netaddr.FromStdAddr(underlayStd.IP, underlayStd.Port, underlayStd.Zone)
		if !ok {
			continue // ignore bad next hop
		}
		paths[i] = &Path{
			Source:      h.ia,
			Destination: dst,
			Metadata:    metadata,
			Fingerprint: pathSequenceFromInterfaces(metadata.Interfaces).Fingerprint(),
			Expiry:      snetMetadata.Expiry,
			ForwardingPath: ForwardingPath{
				spath:    p.Path(),
				underlay: underlay,
			},
		}
	}
	return paths, nil
}

func convertPathInterfaceSlice(spis []snet.PathInterface) []PathInterface {
	pis := make([]PathInterface, len(spis))
	for i, spi := range spis {
		pis[i] = PathInterface{
			IA:   IA(spi.IA),
			IfID: IfID(spi.ID),
		}
	}
	return pis
}
