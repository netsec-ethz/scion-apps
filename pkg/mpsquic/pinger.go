// Copyright 2020 ETH Zurich
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

package mpsquic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/topology/overlay"
)

// Pinger sends SCMP echo requests ("pings") and receivs the corresponding SCMP echo replies.
type Pinger struct {
	conn  snet.PacketConn
	laddr *snet.UDPAddr
}

// EchoReply contains information about an SCMP echo reply received from the network.
type EchoReply struct {
	Addr *snet.UDPAddr
	ID   uint64
	Seq  uint16
	RTT  time.Duration
}

// newPinger opens a new connection to send/receive SCMP echo requests/replies on.
func newPinger(ctx context.Context,
	revocationHandler snet.RevocationHandler) (*Pinger, error) {

	conn, laddr, err := listenPacketConn(ctx, &pingerSCMPHandler{revocationHandler})
	if err != nil {
		return nil, err
	}

	return &Pinger{conn, laddr}, nil
}

// Close closes the underlying connection.
func (p *Pinger) Close() error {
	return p.conn.Close()
}

// PingAll sends one SCMP echo request to the given addresses and awaits before the
// timeout. A path must be set on each address.
// The application ID must be unique on this host. This ID is used as the base
// ID for the IDs of the echo requests sent to the individual addresses.
// The user is responsible for choosing unique ids.
// Returns the round trip time to each address (in input order). If no reply
// was received before the timeout, maxDuration is returned.
func (p *Pinger) PingAll(addrs []*snet.UDPAddr, id uint64, seq uint16,
	timeout time.Duration) ([]time.Duration, error) {

	deadline := time.Now().Add(timeout)
	result := make(chan rttsOrErr)
	go p.awaitReplies(len(addrs), id, seq, deadline, result)

	for i, addr := range addrs {
		err := p.Ping(addr, id+uint64(i), seq)
		if err != nil {
			return nil, err
		}
	}

	ret := <-result
	return ret.rtts, ret.err
}

func (p *Pinger) awaitReplies(n int, baseID uint64, seq uint16,
	deadline time.Time, result chan rttsOrErr) {

	rtts, err := p.readReplies(n, baseID, seq, deadline)
	result <- rttsOrErr{rtts: rtts, err: err}
}

type rttsOrErr struct {
	rtts []time.Duration
	err  error
}

func (p *Pinger) readReplies(n int, baseID uint64, seq uint16,
	deadline time.Time) ([]time.Duration, error) {

	rtts := make([]time.Duration, n)
	for i := range rtts {
		rtts[i] = maxDuration
	}
	nReceived := 0
	for nReceived < n && time.Now().Before(deadline) {
		reply, err := p.ReadReply(deadline)
		if err != nil {
			if isTimeout(err) {
				break
			}
			return nil, err
		}
		if reply.ID >= baseID && reply.ID < baseID+uint64(n) &&
			reply.Seq == seq &&
			rtts[reply.ID-baseID] == maxDuration {

			rtts[reply.ID-baseID] = reply.RTT
			nReceived++
		} else {
			logger.Debug("Unexpected SCMP echo reply", "id", reply.ID, "seq", reply.Seq, "baseID", baseID)
		}
	}
	return rtts, nil
}

func isTimeout(err error) bool {
	if be, ok := err.(common.BasicError); ok {
		err = be.Unwrap()
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

// Ping sends an SCMP echo request to the given address. A path must be set.
// The application ID must be unique on this host. The user is responsible for
// choosing unique ids.
func (p *Pinger) Ping(addr *snet.UDPAddr, id uint64, seq uint16) error {
	pkt := newSCMPPkt(
		p.laddr,
		addr,
		scmp.T_G_EchoRequest,
		&scmp.InfoEcho{Id: id, Seq: seq},
	)
	nextHop := addr.NextHop
	if nextHop == nil && p.laddr.IA.Equal(addr.IA) {
		nextHop = &net.UDPAddr{
			IP:   addr.Host.IP,
			Port: overlay.EndhostPort,
			Zone: addr.Host.Zone,
		}
	}
	return p.conn.WriteTo(pkt, nextHop)
}

// ReadReply receives an SCMP echo reply.
// Replies are returned in the order they are received from the network,
// independently of the order in which the requests are sent.
//
// Note: the RTT in the returned struct is determined based on the time the
// packet is processed. If this is handled delayed, e.g. because this method is
// not invoked immediately after sending the Ping, the returned RTT may be biased.
func (p *Pinger) ReadReply(deadline time.Time) (EchoReply, error) {
	var pkt snet.Packet
	var ov net.UDPAddr
	err := p.conn.SetReadDeadline(deadline)
	if err != nil {
		return EchoReply{}, err
	}
	for {
		// ReadFrom reads SCMP and passes it through the scmp handler.
		// On an echo reply, the SCMP handler returns an error containing the echo reply information.
		// This error makes the ReadFrom return, without reading a data packet.
		err := p.conn.ReadFrom(&pkt, &ov)
		var errEcho *echoError
		if err == nil {
			// Ignore data packets
			continue
		} else if errors.As(err, &errEcho) {
			return errEcho.EchoReply, nil
		} else {
			return EchoReply{}, err
		}
	}
}

// echoError is a pseudo-error returned on an echo reply by the pingerSCMPHandler.
// This is not an error, but the error in SCIONPacketConn.ReadFrom is (ab-)used
// to pass the echo information back to Pinger.ReadReply.
type echoError struct {
	EchoReply
}

func (e *echoError) Error() string {
	return fmt.Sprintf("echo reply received src=%s, id=%d, seq=%d, rtt=%v",
		e.Addr, e.ID, e.Seq, e.RTT)
}

type pingerSCMPHandler struct {
	revocationHandler snet.RevocationHandler
}

func (h *pingerSCMPHandler) Handle(pkt *snet.Packet) error {
	hdr, ok := pkt.L4Header.(*scmp.Hdr)
	if !ok {
		return common.NewBasicError("scmp handler invoked with non-scmp packet", nil, "pkt", pkt)
	}
	pld, ok := pkt.Payload.(*scmp.Payload)
	if !ok {
		return common.NewBasicError("scmp handler invoked with non-scmp payload", nil,
			"type", common.TypeOf(pkt.Payload))
	}

	switch info := pld.Info.(type) {
	case *scmp.InfoRevocation:
		h.revocationHandler.RevokeRaw(context.Background(), info.RawSRev)
		return nil
	case *scmp.InfoEcho:
		return h.handleEcho(&pkt.Source, pkt.Path, hdr, info)
	default:
		logger.Debug("Ignoring scmp packet", "hdr", hdr, "src", pkt.Source)
		return nil
	}
}

func (h *pingerSCMPHandler) handleEcho(src *snet.SCIONAddress, path *spath.Path,
	hdr *scmp.Hdr, info *scmp.InfoEcho) error {

	rtt := time.Since(hdr.Time()).Round(time.Microsecond)
	err := path.Reverse()
	if err != nil {
		return err
	}
	addr := &snet.UDPAddr{IA: src.IA, Path: path, Host: &net.UDPAddr{IP: src.Host.IP()}}
	return &echoError{
		EchoReply{
			Addr: addr,
			ID:   info.Id,
			Seq:  info.Seq,
			RTT:  rtt,
		},
	}
}

// listenPacketConn is analogous to appnet.Listen(nil), but creates a (low-level)
// snet.PacketConn instead of the (high-level) snet.Conn.
func listenPacketConn(ctx context.Context,
	scmpHandler snet.SCMPHandler) (snet.PacketConn, *snet.UDPAddr, error) {

	disp := appnet.DefNetwork().Dispatcher
	localIA := appnet.DefNetwork().IA
	localIP, err := appnet.DefaultLocalIP()
	if err != nil {
		return nil, nil, err
	}
	dispConn, port, err := disp.Register(ctx, localIA, &net.UDPAddr{IP: localIP, Port: 0},
		addr.SvcNone)
	if err != nil {
		return nil, nil, err
	}
	laddr := &snet.UDPAddr{IA: localIA, Host: &net.UDPAddr{IP: localIP, Port: int(port)}}
	return snet.NewSCIONPacketConn(dispConn, scmpHandler), laddr, nil
}

func newSCMPPkt(src, dst *snet.UDPAddr, t scmp.Type, info scmp.Info) *snet.Packet {

	scmpMeta := scmp.Meta{InfoLen: uint8(info.Len() / common.LineLen)}
	pld := make(common.RawBytes, scmp.MetaLen+info.Len())
	err := scmpMeta.Write(pld)
	if err != nil {
		panic(err)
	}
	_, err = info.Write(pld[scmp.MetaLen:])
	if err != nil {
		panic(err)
	}
	scmpHdr := scmp.NewHdr(scmp.ClassType{Class: scmp.C_General, Type: t}, len(pld))
	return &snet.Packet{
		PacketInfo: snet.PacketInfo{
			Source:      snet.SCIONAddress{IA: src.IA, Host: addr.HostFromIP(src.Host.IP)},
			Destination: snet.SCIONAddress{IA: dst.IA, Host: addr.HostFromIP(dst.Host.IP)},
			Path:        dst.Path,
			L4Header:    scmpHdr,
			Payload:     pld,
		},
	}
}
