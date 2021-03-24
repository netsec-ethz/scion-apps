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
	"strings"
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/slayers"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/topology/underlay"
)

// openBaseUDPConn opens new raw SCION UDP conn.
func openBaseUDPConn(ctx context.Context, local *net.UDPAddr) (snet.PacketConn, UDPAddr, error) {
	dispatcher := host().dispatcher
	ia := host().ia

	rconn, port, err := dispatcher.Register(ctx, addr.IA(ia), local, addr.SvcNone)
	if err != nil {
		return nil, UDPAddr{}, err
	}
	conn := snet.NewSCIONPacketConn(rconn, scmpHandler{}, true)
	slocal := UDPAddr{
		IA:   ia,
		IP:   local.IP,
		Port: int(port),
	}
	return conn, slocal, nil
}

// baseUDPConn contains the common message read/write logic for different the
// UDP porcelains (dialedConn and listenConn).
// Currently this wraps snet.PacketConn/snet.SCIONPacketConn, but this logic
// could easily be moved here too.
type baseUDPConn struct {
	raw         snet.PacketConn
	readMutex   sync.Mutex
	readBuffer  snet.Bytes
	writeMutex  sync.Mutex
	writeBuffer snet.Bytes
}

func (c *baseUDPConn) SetDeadline(t time.Time) error {
	return c.raw.SetDeadline(t)
}

func (c *baseUDPConn) SetReadDeadline(t time.Time) error {
	return c.raw.SetReadDeadline(t)
}

func (c *baseUDPConn) SetWriteDeadline(t time.Time) error {
	return c.raw.SetWriteDeadline(t)
}

func (c *baseUDPConn) writeMsg(src, dst UDPAddr, path *Path, b []byte) (int, error) {
	// assert:
	if src.IA != path.Source {
		panic("writeMsg: src.IA != path.Source")
	}
	if dst.IA != path.Destination {
		panic("writeMsg: dst.IA != path.Destination")
	}

	var spath spath.Path
	var nextHop *net.UDPAddr
	if src.IA == dst.IA {
		nextHop = &net.UDPAddr{
			IP:   dst.IP,
			Port: underlay.EndhostPort,
		}
	} else {
		nextHop = path.ForwardingPath.underlay
		spath = path.ForwardingPath.spath
	}

	pkt := &snet.Packet{
		Bytes: c.writeBuffer,
		PacketInfo: snet.PacketInfo{ // bah
			Source: snet.SCIONAddress{
				IA:   addr.IA(src.IA),
				Host: addr.HostFromIP(src.IP),
			},
			Destination: snet.SCIONAddress{
				IA:   addr.IA(dst.IA),
				Host: addr.HostFromIP(dst.IP),
			},
			Path: spath,
			Payload: snet.UDPPayload{
				SrcPort: uint16(src.Port),
				DstPort: uint16(dst.Port),
				Payload: b,
			},
		},
	}

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	err := c.raw.WriteTo(pkt, nextHop)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// readMsg is a helper for reading a single packet.
// Internally invokes the configured SCMP handler.
// Ignores non-UDP packets.
func (c *baseUDPConn) readMsg(b []byte) (int, UDPAddr, ForwardingPath, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	for {
		pkt := snet.Packet{
			Bytes: c.readBuffer,
		}
		var lastHop net.UDPAddr
		err := c.raw.ReadFrom(&pkt, &lastHop)
		if err != nil {
			// FIXME:HACK: snet does not properly parse all SCMP types; just do *something*
			if strings.HasPrefix(err.Error(), "decoding packet\n    unhandled SCMP type") {
				// cant get this out without parsing the string
				err = SCMPError{
					typeCode: slayers.CreateSCMPTypeCode(
						slayers.SCMPTypeParameterProblem,
						slayers.SCMPCodeInvalidPacketSize,
					),
				}
			}
			return 0, UDPAddr{}, ForwardingPath{}, err
		}
		udp, ok := pkt.Payload.(snet.UDPPayload)
		if !ok {
			continue // ignore non-UDP packet
		}
		remote := UDPAddr{
			IA:   IA(pkt.Source.IA),
			IP:   append(net.IP{}, pkt.Source.Host.IP()...),
			Port: int(udp.SrcPort),
		}
		fw := ForwardingPath{
			spath:    pkt.Path.Copy(),
			underlay: &lastHop,
		}
		n := copy(b, udp.Payload)
		return n, remote, fw, nil
	}
}

func (c *baseUDPConn) Close() error {
	return c.raw.Close()
}

type scmpHandler struct{}

func (h scmpHandler) Handle(pkt *snet.Packet) error {
	scmp := pkt.Payload.(snet.SCMPPayload)
	switch scmp.Type() {
	case slayers.SCMPTypeExternalInterfaceDown:
		msg := pkt.Payload.(snet.SCMPExternalInterfaceDown)
		pi := PathInterface{
			IA:   IA(msg.IA),
			IfID: IfID(msg.Interface),
		}
		p, err := reversePathFromForwardingPath(
			IA(pkt.Destination.IA), // the local IA
			IA{},                   // original destination unknown, would require parsing the SCMP quote
			ForwardingPath{spath: pkt.Path},
		)
		if err != nil { // bad packet, drop silently
			return nil
		}
		// FIXME: can block _all_ connections, call async (or internally async)
		stats.NotifyPathDown(p.Fingerprint, pi)
		return nil
	case slayers.SCMPTypeInternalConnectivityDown:
		msg := pkt.Payload.(snet.SCMPInternalConnectivityDown)
		pi := PathInterface{
			IA:   IA(msg.IA),
			IfID: IfID(msg.Egress),
		}
		p, err := reversePathFromForwardingPath(
			IA(pkt.Destination.IA), // the local IA
			IA{},                   // unknown
			ForwardingPath{spath: pkt.Path},
		)
		if err != nil {
			return nil
		}
		stats.NotifyPathDown(p.Fingerprint, pi)
		return nil
	default:
		return SCMPError{
			typeCode: slayers.CreateSCMPTypeCode(scmp.Type(), scmp.Code()),
			ErrorIA:  IA(pkt.Source.IA),
			ErrorIP:  append(net.IP{}, pkt.Source.Host.IP()...),
		}
	}
}

type SCMPError struct {
	typeCode slayers.SCMPTypeCode
	// ErrorIA is the source IA of the SCMP error message
	ErrorIA IA
	// ErrorIP is the source IP of the SCMP error message
	ErrorIP net.IP
	// TODO: include quote information (pkt destinition, path, ...)
}

func (e SCMPError) Error() string {
	return fmt.Sprintf("SCMP %s from %s,%s", e.typeCode.String(), e.ErrorIA, e.ErrorIP)
}

func (e SCMPError) Temporary() bool {
	switch e.typeCode.Type() {
	case slayers.SCMPTypeDestinationUnreachable:
	case slayers.SCMPTypePacketTooBig:
	case slayers.SCMPTypeParameterProblem:
		return false
	case slayers.SCMPTypeExternalInterfaceDown:
	case slayers.SCMPTypeInternalConnectivityDown:
		return true
	}
	panic("invalid error code")
}
