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
	golog "log"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	"github.com/scionproto/scion/pkg/log"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/scionproto/scion/pkg/snet/addrutil"
	"github.com/scionproto/scion/private/app/flag"
)

// hostContext contains the information needed to connect to the host's local SCION stack,
// i.e. the connection to sciond.
type hostContext struct {
	ia       addr.IA
	scionD   daemon.Connector
	topology snet.Topology
	addr     netip.Addr

	interfaces *atomic.Pointer[map[uint16]netip.AddrPort]
}

type ASContext interface {
	// IA returns the local ISD-AS.
	IA() addr.IA
	// Topology returns the local AS topology.
	Topology() snet.Topology
	// Paths returns the available paths to the given destination IA.
	Paths(ctx context.Context, dst addr.IA) ([]*Path, error)
	// LocalAddr returns the local IP address to use.
	LocalAddr() netip.Addr
}

const (
	initTimeout = 1 * time.Second
)

// MustLoadDefaultASContext loads the ASContext from the environment by
// connecting to the local SCIOND and initializes the default path pool.
// If loading fails, it fatals.
//
// This is a convenience function for simple applications and will be
// obsolete once the new client API is in place.
func MustLoadDefaultASContext() ASContext {
	asCtx, err := LoadASContext(context.Background())
	if err != nil {
		golog.Fatalf("Failed to load AS context: %v", err)
	}
	PathPoolInit(asCtx, DefaultPathPoolConfig())

	return asCtx
}

func LoadASContext(ctx context.Context) (ASContext, error) {
	var scionEnv flag.SCIONEnvironment
	if err := scionEnv.LoadExternalVars(); err != nil {
		return nil, fmt.Errorf("unable to load SCION environment: %w", err)
	}
	log.FromCtx(ctx).Debug("SCION environment loaded", "daemon", scionEnv.Daemon(),
		"local", scionEnv.Local(),
	)
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	sciondConn, err := daemon.NewService(scionEnv.Daemon()).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to connect to SCIOND at %s (override with SCION_DAEMON): %w",
			scionEnv.Daemon(), err,
		)
	}
	localIA, err := sciondConn.LocalIA(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get local ISD-AS from SCIOND: %w", err)
	}
	start, end, err := sciondConn.PortRange(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get port range from SCIOND: %w", err)
	}
	interfaces, err := sciondConn.Interfaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get interfaces from SCIOND: %w", err)
	}
	localAddr := scionEnv.Local()
	if !scionEnv.Local().IsValid() {
		localIP, err := addrutil.DefaultLocalIP(ctx, daemon.TopoQuerier{Connector: sciondConn})
		if err != nil {
			return nil, fmt.Errorf("unable to determine local IP: %w", err)
		}
		var ok bool
		localAddr, ok = netip.AddrFromSlice(localIP)
		if !ok {
			return nil, fmt.Errorf(
				"unable to convert local IP to netip.Addr: %v",
				localIP,
			)
		}
	}
	localAddr = localAddr.Unmap()
	hc := hostContext{
		ia:         localIA,
		scionD:     sciondConn,
		addr:       localAddr,
		interfaces: &atomic.Pointer[map[uint16]netip.AddrPort]{},
	}
	hc.interfaces.Store(&interfaces)
	hc.topology = snet.Topology{
		LocalIA: localIA,
		PortRange: snet.TopologyPortRange{
			Start: start,
			End:   end,
		},
		Interface: func(ifID uint16) (netip.AddrPort, bool) {
			m := hc.interfaces.Load()
			if m == nil {
				return netip.AddrPort{}, false
			}
			a, ok := (*m)[ifID]
			return a, ok
		},
	}

	return &hc, nil
}

func (h *hostContext) IA() addr.IA {
	return h.ia
}

func (h *hostContext) Topology() snet.Topology {
	return h.topology
}

func (h *hostContext) LocalAddr() netip.Addr {
	return h.addr
}

func (h *hostContext) Paths(ctx context.Context, dst addr.IA) ([]*Path, error) {
	flags := daemon.PathReqFlags{Refresh: false, Hidden: false}
	snetPaths, err := h.scionD.Paths(ctx, dst, 0, flags)
	if err != nil {
		return nil, err
	}
	// when querying paths we also reload the interfaces, to make sure we have
	// correct information about them.
	interfaces, err := h.scionD.Interfaces(ctx)
	if err != nil {
		return nil, err
	}
	h.interfaces.Store(&interfaces)
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
		underlay := p.UnderlayNextHop().AddrPort()
		paths[i] = &Path{
			Source:      h.ia,
			Destination: dst,
			Metadata:    metadata,
			Fingerprint: PathSequenceFromInterfaces(metadata.Interfaces).Fingerprint(),
			Expiry:      snetMetadata.Expiry,
			ForwardingPath: ForwardingPath{
				dataplanePath: p.Dataplane(),
				underlay:      underlay,
			},
		}
	}
	return paths, nil
}

func convertPathInterfaceSlice(spis []snet.PathInterface) []PathInterface {
	pis := make([]PathInterface, len(spis))
	for i, spi := range spis {
		pis[i] = PathInterface{
			IA:   spi.IA,
			IfID: IfID(spi.ID),
		}
	}
	return pis
}
