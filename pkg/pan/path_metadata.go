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
	"fmt"
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/snet"
)

type PathInterface struct {
	IA   IA
	IfID IfID
}

// PathMetadata contains supplementary information about a path.
//
// The information about MTU, Latency, Bandwidth etc. are based solely on data
// contained in the AS entries in the path construction beacons. These entries
// are signed/verified based on the control plane PKI. However, the
// *correctness* of this meta data has *not* been checked.
//
// NOTE: copied from snet.PathMetadata: does not contain Expiry and uses the
// local types (pan.PathInterface instead of snet.PathInterface, ...)
type PathMetadata struct {
	// Interfaces is a list of interfaces on the path.
	Interfaces []PathInterface

	// MTU is the maximum transmission unit for the path, in bytes.
	MTU uint16

	// Latency lists the latencies between any two consecutive interfaces.
	// Entry i describes the latency between interface i and i+1.
	// Consequently, there are N-1 entries for N interfaces.
	// A 0-value indicates that the AS did not announce a latency for this hop.
	Latency []time.Duration

	// Bandwidth lists the bandwidth between any two consecutive interfaces, in Kbit/s.
	// Entry i describes the bandwidth between interfaces i and i+1.
	// A 0-value indicates that the AS did not announce a bandwidth for this hop.
	Bandwidth []uint64

	// Geo lists the geographical position of the border routers along the path.
	// Entry i describes the position of the router for interface i.
	// A 0-value indicates that the AS did not announce a position for this router.
	Geo []GeoCoordinates

	// LinkType contains the announced link type of inter-domain links.
	// Entry i describes the link between interfaces 2*i and 2*i+1.
	LinkType []LinkType

	// InternalHops lists the number of AS internal hops for the ASes on path.
	// Entry i describes the hop between interfaces 2*i+1 and 2*i+2 in the same AS.
	// Consequently, there are no entries for the first and last ASes, as these
	// are not traversed completely by the path.
	InternalHops []uint32

	// Notes contains the notes added by ASes on the path, in the order of occurrence.
	// Entry i is the note of AS i on the path.
	Notes []string
}

type GeoCoordinates = snet.GeoCoordinates
type LinkType = snet.LinkType

func (pm *PathMetadata) Copy() *PathMetadata {
	if pm == nil {
		return nil
	}
	return &PathMetadata{
		Interfaces:   append(pm.Interfaces[:0:0], pm.Interfaces...),
		MTU:          pm.MTU,
		Latency:      append(pm.Latency[:0:0], pm.Latency...),
		Bandwidth:    append(pm.Bandwidth[:0:0], pm.Bandwidth...),
		Geo:          append(pm.Geo[:0:0], pm.Geo...),
		LinkType:     append(pm.LinkType[:0:0], pm.LinkType...),
		InternalHops: append(pm.InternalHops[:0:0], pm.InternalHops...),
		Notes:        append(pm.Notes[:0:0], pm.Notes...),
	}
}

// LowerLatency compares the latency of two paths.
// Returns
//  - true, true if a has strictly lower latency than b
//  - false, true if a has equal or higher latency than b
//  - _, false if not enough information is available to compare a and b
func (pm *PathMetadata) LowerLatency(b *PathMetadata) (bool, bool) {
	totA, unknownA := pm.latencySum()
	totB, unknownB := b.latencySum()
	if totA < totB && unknownA.subsetOf(unknownB) {
		// total of known smaller and all unknown hops in A are also in B
		return true, true
	} else if totA >= totB && unknownB.subsetOf(unknownA) {
		// total of known larger/equal all unknown hops in B are also in A
		return false, true
	}
	return false, false
}

// latencySum returns the total latency and the set of edges with unknown
// latency
// NOTE: the latency from the end hosts to the first/last interface is always
// unknown. If that would be taken into account, all the paths become
// incomparable.
func (pm *PathMetadata) latencySum() (time.Duration, pathHopSet) {
	var sum time.Duration
	unknown := make(pathHopSet)
	for i := 0; i < len(pm.Interfaces)-1; i++ {
		l := pm.Latency[i]
		if l != 0 { // FIXME: needs to be fixed in combinator/snet; should not use 0 for unknown
			sum += l
		} else {
			unknown[pathHop{a: pm.Interfaces[i], b: pm.Interfaces[i+1]}] = struct{}{}
		}
	}
	return sum, unknown
}

func (pm *PathMetadata) fmtInterfaces() string {
	if len(pm.Interfaces) == 0 {
		return ""
	}
	b := &strings.Builder{}
	intf := pm.Interfaces[0]
	fmt.Fprintf(b, "%s %d", intf.IA, intf.IfID)
	for i := 1; i < len(pm.Interfaces)-1; i += 2 {
		inIntf := pm.Interfaces[i]
		outIntf := pm.Interfaces[i+1]
		fmt.Fprintf(b, ">%d %s %d", inIntf.IfID, inIntf.IA, outIntf.IfID)
	}
	intf = pm.Interfaces[len(pm.Interfaces)-1]
	fmt.Fprintf(b, ">%d %s", intf.IfID, intf.IA)
	return b.String()
}

type pathHop struct {
	a, b PathInterface
}

type pathHopSet map[pathHop]struct{}

func (a pathHopSet) subsetOf(b pathHopSet) bool {
	for x := range a {
		if _, inB := b[x]; !inB {
			return false
		}
	}
	return true
}

func isInterfaceOnPath(p *Path, pi PathInterface) bool {
	for _, c := range p.Metadata.Interfaces {
		if c == pi {
			return true
		}
	}
	return false
}
