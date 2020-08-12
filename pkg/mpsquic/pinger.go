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

const replyBufferCapacity = 128

// Pinger sends SCMP echo requests ("pings") and receives the corresponding SCMP echo replies.
type Pinger struct {
	// Replies are returned in the order they are received from the network,
	// independently of the order in which the requests are sent.
	Replies <-chan EchoReply
	conn    snet.PacketConn
	laddr   *snet.UDPAddr
	stop    chan struct{}
}

// EchoReply contains information about an SCMP echo reply received from the network.
type EchoReply struct {
	Addr *snet.UDPAddr
	ID   uint64
	Seq  uint16
	RTT  time.Duration
}

// newPinger opens a new connection to send/receive SCMP echo requests/replies on.
func NewPinger(ctx context.Context, revHandler snet.RevocationHandler) (*Pinger, error) {

	replies := make(chan EchoReply, replyBufferCapacity)

	scmpHandler := &pingerSCMPHandler{
		revHandler: revHandler,
		replies:    replies,
	}

	conn, laddr, err := listenPacketConn(ctx, scmpHandler)
	if err != nil {
		return nil, err
	}

	p := &Pinger{
		Replies: replies,
		conn:    conn,
		laddr:   laddr,
		stop:    make(chan struct{}),
	}
	go p.drain()
	return p, nil
}

// Close closes the underlying connection.
func (p *Pinger) Close() error {
	close(p.stop)
	return p.conn.Close()
}

// PingAll sends one SCMP echo request to the given addresses and awaits echo
// replies until the timeout.
// A path must be set on each address.
// The application ID must be unique on this host. This ID is used as the base
// ID for the IDs of the echo requests sent to the individual addresses.
// The user is responsible for choosing unique ids.
// Returns the round trip time to each address (in input order). If no reply
// was received before the timeout, maxDuration is returned.
func (p *Pinger) PingAll(addrs []*snet.UDPAddr, id uint64, seq uint16,
	timeout time.Duration) ([]time.Duration, error) {

	for i, addr := range addrs {
		err := p.Ping(addr, id+uint64(i), seq)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.readReplies(ctx, addrs, id, seq)
}

func (p *Pinger) readReplies(ctx context.Context,
	addrs []*snet.UDPAddr, baseID uint64, seq uint16) ([]time.Duration, error) {

	n := len(addrs)
	rtts := make([]time.Duration, n)
	for i := range rtts {
		rtts[i] = maxDuration
	}
	nReceived := 0
	for nReceived < n {
		select {
		case <-ctx.Done():
			return rtts, nil
		case reply := <-p.Replies:
			if reply.ID >= baseID && reply.ID < baseID+uint64(n) &&
				reply.Seq == seq &&
				rtts[reply.ID-baseID] == maxDuration {

				rtts[reply.ID-baseID] = reply.RTT
				nReceived++
			} else {
				logger.Debug("Unexpected SCMP echo reply", "id", reply.ID, "seq",
					reply.Seq, "baseID", baseID)
			}
		}
	}
	return rtts, nil
}

// Ping sends an SCMP echo request to the given address. A path must be set.
// The application ID must be unique on this host. The user is responsible for
// choosing unique ids.
func (p *Pinger) Ping(addr *snet.UDPAddr, id uint64, seq uint16) error {
	pkt := newEchoRequest(p.laddr, addr, id, seq)
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

// drain reads packets from the connection.
// When receiving SCMPs, this triggers the pingerScmpHandler, which registers
// the timestamp and inserts the reply in the (buffered) channel.
func (p *Pinger) drain() {
	var pkt snet.Packet
	var ov net.UDPAddr
	for {
		select {
		case <-p.stop:
			break
		default:
			err := p.conn.ReadFrom(&pkt, &ov)
			if err != nil {
				return
			}
		}
	}
}

type pingerSCMPHandler struct {
	revHandler snet.RevocationHandler
	replies    chan<- EchoReply
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
		h.revHandler.RevokeRaw(context.Background(), info.RawSRev)
		return nil
	case *scmp.InfoEcho:
		err := h.handleEcho(&pkt.Source, pkt.Path, hdr, info)
		if err != nil {
			logger.Debug("Ignoring invalid echo reply", "hdr", hdr, "src", pkt.Source, "info", info)
		}
		return nil
	default:
		logger.Debug("Ignoring scmp packet", "hdr", hdr, "src", pkt.Source, "info", info)
		return nil
	}
}

// handleEcho records the round trip time and inserts an EchoReply to the
// buffered reply channel.
// Note: the RTT in the returned struct is determined based on the time the
// packet is processed. If this is handled delayed, e.g. because the packets
// are not drained fast enough, the recorded RTT may be biased.
func (h *pingerSCMPHandler) handleEcho(src *snet.SCIONAddress, path *spath.Path,
	hdr *scmp.Hdr, info *scmp.InfoEcho) error {

	rtt := time.Since(hdr.Time()).Round(time.Microsecond)
	err := path.Reverse()
	if err != nil {
		return err
	}
	addr := &snet.UDPAddr{IA: src.IA, Path: path, Host: &net.UDPAddr{IP: src.Host.IP()}}
	reply := EchoReply{
		Addr: addr,
		ID:   info.Id,
		Seq:  info.Seq,
		RTT:  rtt,
	}
	select {
	case h.replies <- reply:
	default:
		// buffer full, drop it
	}
	return nil
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

func newEchoRequest(src, dst *snet.UDPAddr, id uint64, seq uint16) *snet.Packet {

	info := &scmp.InfoEcho{Id: id, Seq: seq}
	meta := scmp.Meta{InfoLen: uint8(info.Len() / common.LineLen)}
	pld := make(common.RawBytes, scmp.MetaLen+info.Len())
	err := meta.Write(pld)
	if err != nil {
		panic(err)
	}
	_, err = info.Write(pld[scmp.MetaLen:])
	if err != nil {
		panic(err)
	}
	return &snet.Packet{
		PacketInfo: snet.PacketInfo{
			Source: snet.SCIONAddress{
				IA:   src.IA,
				Host: addr.HostFromIP(src.Host.IP),
			},
			Destination: snet.SCIONAddress{
				IA:   dst.IA,
				Host: addr.HostFromIP(dst.Host.IP),
			},
			Path: dst.Path,
			L4Header: scmp.NewHdr(
				scmp.ClassType{
					Class: scmp.C_General,
					Type:  scmp.T_G_EchoRequest,
				},
				len(pld),
			),
			Payload: pld,
		},
	}
}
