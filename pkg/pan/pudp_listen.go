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
	"errors"
	"net"
	"sync"
	"time"
)

const pudpMaxRaceSequenceNums = 5

var (
	errDupRacePkt = errors.New("race packet sequence number already seen before")
)

func ListenPUDP(ctx context.Context, local *net.UDPAddr) (net.PacketConn, error) {
	controller := &pudpListenerController{
		remotes: make(map[udpAddrKey]pudpRemoteEntry),
		pong:    make(chan pudpPongTask, 128),
		stop:    make(chan struct{}),
	}
	udpConn, err := ListenUDP(ctx, local, controller)
	if err != nil {
		return nil, err
	}
	go controller.Run()
	return &unconnectedPUDPConn{
		listenConn: udpConn.(*listenConn),
		controller: controller,
	}, nil
}

type unconnectedPUDPConn struct {
	*listenConn // XXX: not sure this is a good idea. Maybe just base it on the scionUDPConn directly

	controller *pudpListenerController
}

func (c *unconnectedPUDPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	for {
		nr, remote, path, err := c.listenConn.ReadFromPath(b)
		if err != nil {
			return 0, nil, err
		}
		v := &pudpListenerControllerPacketVisitor{}
		err = pudpParseHeader(b[:nr], v)
		if err != nil {
			continue
		}
		err = c.controller.registerPacket(remote, path, v)
		if err != nil {
			continue
		}
		n := copy(b, v.pld)
		return n, remote, nil
	}
}

func (c *unconnectedPUDPConn) WriteTo(b []byte, dst net.Addr) (int, error) {
	sdst, ok := dst.(UDPAddr)
	if !ok {
		return 0, errBadDstAddress
	}
	path, header := c.controller.decide(c.local, sdst)
	msg := append(header, b...)
	return c.baseUDPConn.writeMsg(c.local, sdst, path, msg)
}

func (c *unconnectedPUDPConn) Close() error {
	c.controller.Close()
	return c.listenConn.Close()
}

type pudpListenerController struct {
	identity []IfID

	mtx     sync.RWMutex
	remotes map[udpAddrKey]pudpRemoteEntry

	pong chan pudpPongTask
	stop chan struct{}
}

func (c *pudpListenerController) decide(src, dst UDPAddr) (*Path, []byte) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	header := pudpHeaderBuilder{} // TODO use some buffer somewhere?
	if src.IA == dst.IA {
		header.buf.WriteByte(byte(pudpHeaderPayload))
		return nil, header.buf.Bytes()
	}

	remoteEntry, ok := c.remotes[makeKey(dst)]
	if !ok {
		header.buf.WriteByte(byte(pudpHeaderPayload))
		return nil, header.buf.Bytes()
	}
	path := remoteEntry.path
	if remoteEntry.identifierReq && len(c.identity) > 0 {
		header.me(c.identity)
	}
	header.buf.WriteByte(byte(pudpHeaderPayload))
	return path, header.buf.Bytes()
}

func (c *pudpListenerController) ReplyPath(src, dst UDPAddr) *Path {
	panic("not implemented") // not called; pudp overrides WriteTo and we call custom methods below
}

func (c *pudpListenerController) OnPacketReceived(src UDPAddr, dst UDPAddr, path *Path) {
	panic("not implemented") // not called; pudp overrides ReadFrom and we call custom methods below
}

func (c *pudpListenerController) OnPathDown(_ PathFingerprint, _ PathInterface) {
	panic("not implemented") // TODO: Implement
}

func (c *pudpListenerController) Run() {
	probeTimer := time.NewTimer(0)
	<-probeTimer.C
	for {
		select {
		case <-c.stop:
			break
		case <-probeTimer.C:
			// TODO
		}
	}
}

func (c *pudpListenerController) Close() error {
	c.stop <- struct{}{}
	return nil
}

func (c *pudpListenerController) registerPacket(remote UDPAddr, path *Path,
	pkt *pudpListenerControllerPacketVisitor) error {

	c.mtx.Lock()
	defer c.mtx.Unlock()
	remoteKey := makeKey(remote)
	r := c.remotes[remoteKey]

	// replace / add path
	r.addAvailablePath(path)

	if pkt.pingSequenceNum != nil {
		c.pong <- pudpPongTask{
			remote: remote,
			path:   path,
			seq:    pkt.pingSequenceNum.(uint16),
		}
	}

	// send identifiers as long as it keeps being requested
	r.identifierReq = pkt.identifierReq

	// check if this is a duplicate that should be suppressed:
	var raceOutOfOrder bool
	if pkt.raceSequenceNum != nil {
		dup, outOfOrder := r.checkRaceSequenceNum(pkt.raceSequenceNum.(uint16))
		if dup {
			return errDupRacePkt
		}
		raceOutOfOrder = outOfOrder
	}

	// use as current path if packet carries payload (and is not an out-of-order race packet)
	if pkt.pld != nil && !raceOutOfOrder {
		r.path = path
	}

	c.remotes[remoteKey] = r

	return nil
}

type pudpRemoteEntry struct {
	paths            []*Path
	path             *Path
	identifierReq    bool
	raceSequenceNums []uint16
}

func (r *pudpRemoteEntry) addAvailablePath(path *Path) {
	exists := false
	for i, p := range r.paths {
		if p.Fingerprint == path.Fingerprint {
			if p.Expiry.Before(path.Expiry) {
				r.paths[i] = path
			}
			exists = true
			break
		}
	}
	if !exists {
		r.paths = append(r.paths, path)
	}
}

// checkRaceSequenceNum checks and inserts the given sequence number.
func (r *pudpRemoteEntry) checkRaceSequenceNum(seq uint16) (dup, outOfOrder bool) {
	var exists, lowerExist, higherExist bool
	for _, s := range r.raceSequenceNums {
		if s < seq {
			lowerExist = true
		} else if s > seq {
			higherExist = true
		} else if s == seq {
			exists = true
		}
	}
	dup = exists
	outOfOrder = higherExist
	if len(r.raceSequenceNums) < pudpMaxRaceSequenceNums {
		r.raceSequenceNums = append(r.raceSequenceNums, seq)
	} else if lowerExist {
		// replace smallest
		minIdx := 0
		min := r.raceSequenceNums[0]
		for i, s := range r.raceSequenceNums {
			if s < min {
				minIdx = i
			}
		}
		r.raceSequenceNums[minIdx] = seq
	}

	return dup, outOfOrder
}

type pudpPongTask struct {
	remote UDPAddr
	path   *Path
	seq    uint16
}

type pudpListenerControllerPacketVisitor struct {
	pld             []byte
	identifierReq   bool
	raceSequenceNum interface{} // optional uint16
	pingSequenceNum interface{} // optional uint16
}

func (v *pudpListenerControllerPacketVisitor) payload(b []byte) {
	v.pld = b
}

func (v *pudpListenerControllerPacketVisitor) race(seq uint16) {
	v.raceSequenceNum = seq
}

func (v *pudpListenerControllerPacketVisitor) ping(seq uint16) {
	v.pingSequenceNum = seq
}

func (v *pudpListenerControllerPacketVisitor) pong(seq uint16) {
	// ignore (or error!?)
}

func (v *pudpListenerControllerPacketVisitor) identify() {
	v.identifierReq = true
}

func (v *pudpListenerControllerPacketVisitor) me(ifids []IfID) {
	// ignore (or error!?)
}
