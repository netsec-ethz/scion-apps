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
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/slayers"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/topology/underlay"
	"inet.af/netaddr"
)

// openBaseUDPConn opens new raw SCION UDP conn.
func openBaseUDPConn(ctx context.Context, local netaddr.IPPort) (snet.PacketConn, UDPAddr, error) {
	dispatcher := host().dispatcher
	ia := host().ia

	rconn, port, err := dispatcher.Register(ctx, addr.IA(ia), local.UDPAddr(), addr.SvcNone)
	if err != nil {
		return nil, UDPAddr{}, err
	}
	conn := &snet.SCIONPacketConn{
		Conn:        rconn,
		SCMPHandler: scmpHandler{},
	}
	slocal := UDPAddr{
		IA:   ia,
		IP:   local.IP(),
		Port: port,
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
	readBuffer  []byte
	writeMutex  sync.Mutex
	writeBuffer []byte
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
	if src.IA != dst.IA && path == nil {
		panic("writeMsg: need path when src.IA != dst.IA")
	}
	if path != nil && src.IA != path.Source {
		panic("writeMsg: src.IA != path.Source")
	}
	if path != nil && dst.IA != path.Destination {
		panic("writeMsg: dst.IA != path.Destination")
	}

	var spath spath.Path
	var nextHop netaddr.IPPort
	if src.IA == dst.IA {
		nextHop = netaddr.IPPortFrom(dst.IP, underlay.EndhostPort)
	} else {
		nextHop = path.ForwardingPath.underlay
		spath = path.ForwardingPath.spath
	}

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if c.writeBuffer == nil {
		c.writeBuffer = make([]byte, common.SupportedMTU)
	}

	pkt := &snet.Packet{
		Bytes: c.writeBuffer,
		PacketInfo: snet.PacketInfo{
			Source: snet.SCIONAddress{
				IA:   addr.IA(src.IA),
				Host: addr.HostFromIP(src.IP.IPAddr().IP),
			},
			Destination: snet.SCIONAddress{
				IA:   addr.IA(dst.IA),
				Host: addr.HostFromIP(dst.IP.IPAddr().IP),
			},
			Path: spath,
			Payload: snet.UDPPayload{
				SrcPort: src.Port,
				DstPort: dst.Port,
				Payload: b,
			},
		},
	}

	err := c.raw.WriteTo(pkt, nextHop.UDPAddr())
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
	if c.readBuffer == nil {
		c.readBuffer = make([]byte, common.SupportedMTU)
	}

	for {
		pkt := snet.Packet{
			Bytes: c.readBuffer,
		}
		var lastHop net.UDPAddr
		err := c.raw.ReadFrom(&pkt, &lastHop)
		if err != nil {
			return 0, UDPAddr{}, ForwardingPath{}, err
		}
		udp, ok := pkt.Payload.(snet.UDPPayload)
		if !ok {
			continue // ignore non-UDP packet
		}
		srcIP, ok := netaddr.FromStdIP(pkt.Source.Host.IP())
		if !ok {
			continue // ignore non-IP destination
		}
		remote := UDPAddr{
			IA:   IA(pkt.Source.IA),
			IP:   srcIP,
			Port: udp.SrcPort,
		}
		underlay, ok := netaddr.FromStdAddr(lastHop.IP, lastHop.Port, lastHop.Zone)
		if !ok {
			continue // ignore bad Underlay
		}
		fw := ForwardingPath{
			spath:    pkt.Path.Copy(),
			underlay: underlay,
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
		pf, err := reversePathFingerprint(pkt.Path)
		if err != nil { // bad packet, drop silently
			return nil // nolint:nilerr
		}
		// FIXME: can block _all_ connections, call async (or internally async)
		stats.NotifyPathDown(pf, pi)
		return nil
	case slayers.SCMPTypeInternalConnectivityDown:
		msg := pkt.Payload.(snet.SCMPInternalConnectivityDown)
		pi := PathInterface{
			IA:   IA(msg.IA),
			IfID: IfID(msg.Egress),
		}
		pf, err := reversePathFingerprint(pkt.Path)
		if err != nil {
			return nil // nolint:nilerr
		}
		stats.NotifyPathDown(pf, pi)
		return nil
	default:
		ip, _ := netaddr.FromStdIP(pkt.Source.Host.IP())
		return SCMPError{
			typeCode: slayers.CreateSCMPTypeCode(scmp.Type(), scmp.Code()),
			ErrorIA:  IA(pkt.Source.IA),
			ErrorIP:  ip,
		}
	}
}

type SCMPError struct {
	typeCode slayers.SCMPTypeCode
	// ErrorIA is the source IA of the SCMP error message
	ErrorIA IA
	// ErrorIP is the source IP of the SCMP error message
	ErrorIP netaddr.IP
	// TODO: include quote information (pkt destinition, path, ...)
}

func (e SCMPError) Error() string {
	return fmt.Sprintf("SCMP %s from %s,%s", e.typeCode.String(), e.ErrorIA, e.ErrorIP)
}

func (e SCMPError) Temporary() bool {
	switch e.typeCode.Type() {
	case slayers.SCMPTypeDestinationUnreachable:
		return false
	case slayers.SCMPTypePacketTooBig:
		return false
	case slayers.SCMPTypeParameterProblem:
		return false
	case slayers.SCMPTypeExternalInterfaceDown:
		return true
	case slayers.SCMPTypeInternalConnectivityDown:
		return true
	default:
		panic("invalid error code")
	}
}
