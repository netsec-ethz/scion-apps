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
	"net/netip"
	"strings"
	"time"

	"github.com/scionproto/scion/pkg/slayers/path"
	"github.com/scionproto/scion/pkg/slayers/path/scion"
	"github.com/scionproto/scion/pkg/snet"
	snetpath "github.com/scionproto/scion/pkg/snet/path"
)

// TODO: revisit: pointer or value type? what goes where? should ForwardingPath be exported?
type Path struct {
	Source         IA
	Destination    IA
	ForwardingPath ForwardingPath
	Metadata       *PathMetadata // optional
	Fingerprint    PathFingerprint
	Expiry         time.Time
}

func (p *Path) String() string {
	if p.Metadata != nil {
		return p.Metadata.fmtInterfaces()
	} else {
		return fmt.Sprintf("%s %s %s", p.Source, p.Destination, p.Fingerprint)
	}
}

// ForwardingPath represents a data plane forwarding path.
type ForwardingPath struct {
	dataplanePath snet.DataplanePath
	// NOTE: could have global lookup table with ifID->UDP instead of passing this around.
	// Might also allow to "properly" bind to wildcard (cache correct source address per ifID).
	underlay netip.AddrPort
}

func (p ForwardingPath) forwardingPathInfo() (forwardingPathInfo, error) {
	var raw []byte
	switch dataplanePath := p.dataplanePath.(type) {
	case snet.RawReplyPath:
		switch dataplanePath.Path.Type() {
		case scion.PathType:
			raw = make([]byte, dataplanePath.Path.Len())
			if err := dataplanePath.Path.SerializeTo(raw); err != nil {
				return forwardingPathInfo{}, err
			}
		default:
			return forwardingPathInfo{}, fmt.Errorf("unsupported path type %v inside RawReplyPath", dataplanePath.Path.Type())
		}
	case snet.RawPath:
		switch dataplanePath.PathType {
		case scion.PathType:
			raw = dataplanePath.Raw
		default:
			return forwardingPathInfo{}, fmt.Errorf("unsupported path type %v inside RawPath", dataplanePath.PathType)
		}
	case snetpath.SCION:
		raw = dataplanePath.Raw
	default:
		return forwardingPathInfo{}, fmt.Errorf("unsupported path type %T", p.dataplanePath)
	}
	var sp scion.Decoded
	if err := sp.DecodeFromBytes(raw); err != nil {
		return forwardingPathInfo{}, err
	}
	return forwardingPathInfo{
		expiry:       expiryFromDecoded(sp),
		interfaceIDs: interfaceIDsFromDecoded(sp),
	}, nil
}

// reversePathFromForwardingPath creates a Path for the return direction from the information
// on a received packet.
// The created Path includes fingerprint and expiry information.
func reversePathFromForwardingPath(src, dst IA, fwPath ForwardingPath) (*Path, error) {
	// FIXME: inefficient, decoding twice! Change this to decode and then both
	// reverse and extract fw info
	rp, ok := fwPath.dataplanePath.(snet.RawPath)
	if !ok {
		panic(fmt.Sprintf("cannot reverse path type %T", fwPath.dataplanePath))
	}
	if len(rp.Raw) == 0 {
		return (*Path)(nil), nil
	}
	revPath, err := snet.DefaultReplyPather{}.ReplyPath(rp)
	if err != nil {
		return nil, err
	}
	fwPath.dataplanePath = revPath
	fpi, err := fwPath.forwardingPathInfo()
	if err != nil {
		return nil, err
	}
	fingerprint := pathSequence{InterfaceIDs: fpi.interfaceIDs}.Fingerprint()
	return &Path{
		Source:         dst,
		Destination:    src,
		ForwardingPath: fwPath,
		Expiry:         fpi.expiry,
		Fingerprint:    fingerprint,
	}, nil
}

func reversePathFingerprint(p snet.RawPath) (PathFingerprint, error) {
	fpi, err := ForwardingPath{dataplanePath: p}.forwardingPathInfo()
	if err != nil {
		return "", err
	}
	rpf := pathSequence{InterfaceIDs: fpi.interfaceIDs}.Reversed().Fingerprint()
	return rpf, nil
}

// forwardingPathInfo contains information extracted from a dataplane forwarding path.
type forwardingPathInfo struct {
	expiry       time.Time
	interfaceIDs []IfID
}

func expiryFromDecoded(sp scion.Decoded) time.Time {
	hopExpiry := func(info path.InfoField, hf path.HopField) time.Time {
		ts := time.Unix(int64(info.Timestamp), 0)
		exp := path.ExpTimeToDuration(hf.ExpTime)
		return ts.Add(exp)
	}

	ret := maxTime
	hop := 0
	for i, info := range sp.InfoFields {
		seglen := int(sp.Base.PathMeta.SegLen[i])
		for h := 0; h < seglen; h++ {
			exp := hopExpiry(info, sp.HopFields[hop])
			if exp.Before(ret) {
				ret = exp
			}
			hop++
		}
	}
	return ret
}

func interfaceIDsFromDecoded(sp scion.Decoded) []IfID {
	ifIDs := make([]IfID, 0, 2*len(sp.HopFields)-2*len(sp.InfoFields))

	// return first interface in order of traversal
	first := func(hf path.HopField, consDir bool) IfID {
		if consDir {
			return IfID(hf.ConsIngress)
		} else {
			return IfID(hf.ConsEgress)
		}
	}
	// return second interface in order of traversal
	second := func(hf path.HopField, consDir bool) IfID {
		if consDir {
			return IfID(hf.ConsEgress)
		} else {
			return IfID(hf.ConsIngress)
		}
	}

	hop := 0
	for i, info := range sp.InfoFields {
		seglen := int(sp.Base.PathMeta.SegLen[i])
		for h := 0; h < seglen; h++ {
			if h > 0 || (info.Peer && i == 1) {
				ifIDs = append(ifIDs, first(sp.HopFields[hop], info.ConsDir))
			}
			if h < seglen-1 || (info.Peer && i == 0) {
				ifIDs = append(ifIDs, second(sp.HopFields[hop], info.ConsDir))
			}
			hop++
		}
	}

	return ifIDs
}

// pathSequence describes a path by a sequence of raw interface IDs, _not_
// including any AS information.
// This information can be obtained even from the raw forwarding paths.
// This can be used to identify a path by its hop sequence, regardless of which
// path segments it is created from, _if_ the source AS is fixed.
// The same pathSequence can refer to completely different paths in different
// source ASes.
// NOTE: it would be useful to include source and destination IA of the path
// here to get a more specific identifier, even if multiple ASes would be
// involved (currently not supported anyway). This is currently not included
// because this information cannot be reliably obtained from the incompletely
// parsed SCMP error messages.
type pathSequence struct {
	InterfaceIDs []IfID
}

func pathSequenceFromInterfaces(interfaces []PathInterface) pathSequence {
	ifIDs := make([]IfID, len(interfaces))
	for i, iface := range interfaces {
		ifIDs[i] = iface.IfID
	}
	return pathSequence{
		InterfaceIDs: ifIDs,
	}
}

// Fingerprint returns the pathSequence as a comparable/hashable object (string).
// Currently somewhat human readable, could do simple binary encoding (for brevity).
func (s pathSequence) Fingerprint() PathFingerprint {
	if len(s.InterfaceIDs) == 0 {
		return ""
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "%d", s.InterfaceIDs[0])
	for _, ifID := range s.InterfaceIDs[1:] {
		fmt.Fprintf(b, " %d", ifID)
	}
	return PathFingerprint(b.String())
}

func (s pathSequence) Reversed() pathSequence {
	rev := make([]IfID, len(s.InterfaceIDs))
	l := len(s.InterfaceIDs)
	for i := range rev {
		rev[i] = s.InterfaceIDs[l-i-1]
	}
	return pathSequence{InterfaceIDs: rev}
}

// PathFingerprint is an opaque identifier for a path. It identifies a path by
// the sequence of interface identifiers along the path.
type PathFingerprint string

func pathFingerprints(paths []*Path) []PathFingerprint {
	fingerprints := make([]PathFingerprint, len(paths))
	for i, p := range paths {
		fingerprints[i] = p.Fingerprint
	}
	return fingerprints
}
