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
	ia          addr.IA
	topology    snet.Topology
	addr        netip.Addr
	pathQuerier *PathQuerier
	pool        *PathPool
	stats       *pathStatsDB
}

type ASContext interface {
	// IA returns the local ISD-AS.
	IA() addr.IA
	// Topology returns the local AS topology.
	Topology() snet.Topology
	// LocalAddr returns the local IP address to use.
	LocalAddr() netip.Addr
	// PathPool returns the path pool for this context.
	PathPool() *PathPool
	// Stats returns the path statistics database for this context.
	Stats() *pathStatsDB
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

	// Create path querier for fetching paths from SCIOND
	pathQuerier := NewPathQuerier(localIA, sciondConn, interfaces)

	// Create stats database for path statistics
	stats := newPathStatsDB()

	hc := &hostContext{
		ia:          localIA,
		addr:        localAddr,
		pathQuerier: pathQuerier,
		stats:       &stats,
		topology: snet.Topology{
			LocalIA: localIA,
			PortRange: snet.TopologyPortRange{
				Start: start,
				End:   end,
			},
			Interface: pathQuerier.Interface,
		},
	}
	// Create path pool with pathQuerier as the PathSource.
	hc.pool = NewPathPool(pathQuerier, hc.stats, DefaultPathPoolConfig())

	return hc, nil
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

func (h *hostContext) PathPool() *PathPool {
	return h.pool
}

func (h *hostContext) Stats() *pathStatsDB {
	return h.stats
}
