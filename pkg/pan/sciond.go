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
	"net/netip"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	"github.com/scionproto/scion/pkg/log"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/scionproto/scion/pkg/snet/addrutil"
	"github.com/scionproto/scion/private/app/flag"
)

const (
	initTimeout = 1 * time.Second
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

// PAN is the main entry point for SCION networking with pan.
// It manages the connection to the SCION daemon, path caching, and statistics.
// A PAN must be created with New and should be closed when no longer needed.
type PAN struct {
	hostContext
}

// New creates a new PAN instance by connecting to the local SCION daemon
// and initializing the path pool.
func New(ctx context.Context) (*PAN, error) {
	var scionEnv flag.SCIONEnvironment
	if err := scionEnv.LoadExternalVars(); err != nil {
		return nil, fmt.Errorf("unable to load SCION environment: %w", err)
	}
	log.FromCtx(ctx).Debug("SCION environment loaded", "daemon", scionEnv.Daemon(),
		"local", scionEnv.Local(),
	)
	initCtx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	sciondConn, err := daemon.NewService(scionEnv.Daemon()).Connect(initCtx)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to connect to SCIOND at %s (override with SCION_DAEMON): %w",
			scionEnv.Daemon(), err,
		)
	}
	localIA, err := sciondConn.LocalIA(initCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to get local ISD-AS from SCIOND: %w", err)
	}
	start, end, err := sciondConn.PortRange(initCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to get port range from SCIOND: %w", err)
	}
	interfaces, err := sciondConn.Interfaces(initCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to get interfaces from SCIOND: %w", err)
	}
	localAddr := scionEnv.Local()
	if !scionEnv.Local().IsValid() {
		localIP, err := addrutil.DefaultLocalIP(initCtx, daemon.TopoQuerier{Connector: sciondConn})
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

	c := &PAN{
		hostContext: hostContext{
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
		},
	}
	// Create path pool with pathQuerier as the PathSource.
	c.pool = NewPathPool(pathQuerier, c.stats, DefaultPathPoolConfig())

	return c, nil
}

// QueryPaths queries paths to a particular destination AS.
func (p *PAN) QueryPaths(ctx context.Context, dst addr.IA) ([]*Path, error) {
	return p.pool.QueryPaths(ctx, dst)
}
