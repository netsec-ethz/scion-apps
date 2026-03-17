// Copyright 2026 ETH Zurich
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

package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	humm "github.com/scionproto/scion/pkg/hummingbird"
	"github.com/scionproto/scion/pkg/hummingbird/redemption"
	"github.com/scionproto/scion/pkg/segment/iface"
	"github.com/scionproto/scion/pkg/snet"
	snetpath "github.com/scionproto/scion/pkg/snet/path"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

type hummingbirdConfig struct {
	Bw       uint16
	Duration uint16
}

func parseHummingbirdFlag(input string) (*hummingbirdConfig, error) {
	if input == "" {
		return nil, nil
	}
	parts := strings.SplitN(input, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid -hummingbird value %q, expected BW,dur", input)
	}
	bw, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil || bw == 0 {
		return nil, fmt.Errorf("invalid Hummingbird bandwidth %q", parts[0])
	}
	dur, err := time.ParseDuration(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid Hummingbird duration %q: %w", parts[1], err)
	}
	if dur <= 0 {
		return nil, fmt.Errorf("Hummingbird duration must be positive")
	}
	dur.Seconds()
	seconds := dur / time.Second
	if time.Duration(seconds)*time.Second != dur {
		return nil, fmt.Errorf("Hummingbird duration must be a whole number of seconds")
	}
	if seconds > time.Duration(^uint16(0)) {
		return nil, fmt.Errorf("Hummingbird duration too large: %s", dur)
	}
	return &hummingbirdConfig{
		Bw:       uint16(bw),
		Duration: uint16(seconds),
	}, nil
}

func buildHummingbirdPath(
	ctx context.Context,
	basePath *pan.Path,
	localIP netip.Addr,
	cfg *hummingbirdConfig,
) (*pan.Path, error) {
	if cfg == nil {
		return nil, nil
	}
	if basePath == nil {
		return nil, fmt.Errorf("Hummingbird requires an inter-AS SCION path")
	}
	if !localIP.IsValid() {
		return nil, fmt.Errorf("Hummingbird requires a valid local IP")
	}
	sciondConn, err := connectSciond(ctx)
	if err != nil {
		return nil, err
	}
	defer sciondConn.Close()

	reservation, err := redemption.OneShotReservation(
		ctx,
		sciondConn,
		net.IP(localIP.AsSlice()),
		panPathToSnetPath(basePath),
		humm.RedemptionRequestNoHop{
			StartTime: uint32(time.Now().Unix()),
			Bw:        cfg.Bw,
			Duration:  cfg.Duration,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("building Hummingbird reservation: %w", err)
	}
	return basePath.CloneWithDataplanePath(reservation), nil
}

func connectSciond(ctx context.Context) (daemon.Connector, error) {
	address, ok := os.LookupEnv("SCION_DAEMON_ADDRESS")
	if !ok {
		address = daemon.DefaultAPIAddress
	}
	conn, err := daemon.NewService(address).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SCIOND at %s: %w", address, err)
	}
	return conn, nil
}

func panPathToSnetPath(path *pan.Path) snet.Path {
	if path == nil {
		return nil
	}
	return snetpath.Path{
		Src:           addr.IA(path.Source),
		Dst:           addr.IA(path.Destination),
		DataplanePath: path.Dataplane(),
		NextHop:       net.UDPAddrFromAddrPort(path.UnderlayNextHop()),
		Meta:          panMetadataToSnet(path.Metadata, path.Expiry),
	}
}

func panMetadataToSnet(metadata *pan.PathMetadata, expiry time.Time) snet.PathMetadata {
	if metadata == nil {
		return snet.PathMetadata{Expiry: expiry}
	}
	return snet.PathMetadata{
		Interfaces:           panInterfacesToSnet(metadata.Interfaces),
		MTU:                  metadata.MTU,
		Expiry:               expiry,
		Latency:              append([]time.Duration(nil), metadata.Latency...),
		Bandwidth:            append([]uint64(nil), metadata.Bandwidth...),
		Geo:                  append([]snet.GeoCoordinates(nil), metadata.Geo...),
		LinkType:             append([]snet.LinkType(nil), metadata.LinkType...),
		InternalHops:         append([]uint32(nil), metadata.InternalHops...),
		Notes:                append([]string(nil), metadata.Notes...),
		DiscoveryInformation: panDiscoveryToSnet(metadata.DiscoveryInformation),
	}
}

func panInterfacesToSnet(interfaces []pan.PathInterface) []snet.PathInterface {
	out := make([]snet.PathInterface, len(interfaces))
	for i, pathIface := range interfaces {
		out[i] = snet.PathInterface{
			IA: addr.IA(pathIface.IA),
			ID: iface.ID(pathIface.IfID),
		}
	}
	return out
}

func panDiscoveryToSnet(
	discovery map[pan.IA]pan.DiscoveryInformation,
) map[addr.IA]snet.DiscoveryInformation {
	if discovery == nil {
		return nil
	}
	out := make(map[addr.IA]snet.DiscoveryInformation, len(discovery))
	for ia, info := range discovery {
		out[addr.IA(ia)] = snet.DiscoveryInformation{
			ControlServices:   append([]netip.AddrPort(nil), info.ControlServices...),
			DiscoveryServices: append([]netip.AddrPort(nil), info.DiscoveryServices...),
		}
	}
	return out
}

type hummingbirdWriter struct {
	conn pan.Conn
	path *pan.Path
}

func (w *hummingbirdWriter) Write(b []byte) (int, error) {
	return w.conn.WriteVia(w.path, b)
}
