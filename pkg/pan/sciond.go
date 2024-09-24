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
	"github.com/scionproto/scion/pkg/drkey"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/scionproto/scion/pkg/snet/addrutil"
)

// hostContext contains the information needed to connect to the host's local SCION stack,
// i.e. the connection to sciond.
type hostContext struct {
	ia            IA
	sciond        daemon.Connector
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
func defaultLocalIP() (netip.Addr, error) {
	stdIP, err := addrutil.ResolveLocal(host().hostInLocalAS)
	ip, ok := netip.AddrFromSlice(stdIP)
	if err != nil || !ok {
		return netip.Addr{}, fmt.Errorf("unable to resolve default local address %w", err)
	}
	return ip.Unmap(), nil
}

// defaultLocalAddr fills in a missing or unspecified IP field with defaultLocalIP.
func defaultLocalAddr(local netip.AddrPort) (netip.AddrPort, error) {
	if !local.Addr().IsValid() || local.Addr().IsUnspecified() {
		localIP, err := defaultLocalIP()
		if err != nil {
			return netip.AddrPort{}, err
		}
		local = netip.AddrPortFrom(localIP, local.Port())
	}
	return local, nil
}

func (h *hostContext) queryPaths(ctx context.Context, dst IA) ([]*Path, error) {
	flags := daemon.PathReqFlags{Refresh: false, Hidden: false}
	snetPaths, err := h.sciond.Paths(ctx, addr.IA(dst), 0, flags)
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
			FabridInfo:   snetMetadata.FabridInfo,
		}
		underlay := p.UnderlayNextHop().AddrPort()
		paths[i] = &Path{
			Source:      h.ia,
			Destination: dst,
			Metadata:    metadata,
			Fingerprint: pathSequenceFromInterfaces(metadata.Interfaces).Fingerprint(),
			Expiry:      snetMetadata.Expiry,
			ForwardingPath: ForwardingPath{
				dataplanePath: p.Dataplane(),
				underlay:      underlay,
			},
		}
	}
	return paths, nil
}

func GetDRKeyHostHostKey(ctx context.Context, meta drkey.HostHostMeta) (drkey.HostHostKey, error) {
	return host().sciond.DRKeyGetHostHostKey(ctx, meta)
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
