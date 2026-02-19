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
	"net/netip"
	"sync/atomic"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	"github.com/scionproto/scion/pkg/snet"
)

// PathQuerier queries paths from the SCION daemon and converts them to Path objects.
// It implements PathSource for use with PathPool.
// It also maintains the interface cache which is updated on each path query.
type PathQuerier struct {
	localIA    addr.IA
	scionD     daemon.Connector
	interfaces atomic.Pointer[map[uint16]netip.AddrPort]
}

// NewPathQuerier creates a new PathQuerier with an initial interface set.
func NewPathQuerier(
	localIA addr.IA,
	scionD daemon.Connector,
	initialInterfaces map[uint16]netip.AddrPort,
) *PathQuerier {
	q := &PathQuerier{
		localIA: localIA,
		scionD:  scionD,
	}
	q.interfaces.Store(&initialInterfaces)
	return q
}

// Interface returns the underlay address for the given interface ID.
func (q *PathQuerier) Interface(ifID uint16) (netip.AddrPort, bool) {
	m := q.interfaces.Load()
	if m == nil {
		return netip.AddrPort{}, false
	}
	a, ok := (*m)[ifID]
	return a, ok
}

// Paths queries paths to the destination IA from the SCION daemon.
func (q *PathQuerier) Paths(ctx context.Context, dst addr.IA) ([]*Path, error) {
	flags := daemon.PathReqFlags{Refresh: false, Hidden: false}
	snetPaths, err := q.scionD.Paths(ctx, dst, 0, flags)
	if err != nil {
		return nil, err
	}
	// when querying paths we also reload the interfaces, to make sure we have
	// correct information about them.
	interfaces, err := q.scionD.Interfaces(ctx)
	if err != nil {
		return nil, err
	}
	q.interfaces.Store(&interfaces) //nolint:gosec
	return q.convertPaths(dst, snetPaths), nil
}

func (q *PathQuerier) convertPaths(dst addr.IA, snetPaths []snet.Path) []*Path {
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
			Source:      q.localIA,
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
	return paths
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
