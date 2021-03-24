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
	"fmt"
	"net"
	"sync"
	"time"
)

// DialPUDP creates a connected PUDP conn.
// TODO: Extended dial Config?
func DialPUDP(ctx context.Context, local *net.UDPAddr, remote UDPAddr, policy Policy) (net.Conn, error) {
	controller := &pudpController{
		maxRace: 5, // XXX
		stop:    make(chan struct{}),
	}
	udpConn, err := DialUDP(ctx, local, remote, policy, controller)
	if err != nil {
		return nil, err
	}
	go controller.Run(udpConn)
	return &dialedPUDPConn{
		dialedConn: udpConn.(*dialedConn),
		controller: controller,
	}, nil
}

type dialedPUDPConn struct {
	*dialedConn
	controller *pudpController
}

func (c *dialedPUDPConn) Write(b []byte) (int, error) {
	paths, header := c.controller.decide()
	msg := append(header, b...)
	if len(paths) > 1 {
		fmt.Println("racing", len(paths))
		fmt.Println(paths)
	}
	for _, path := range paths {
		_, _ = c.dialedConn.WritePath(path, msg)
	}
	return len(b), nil // XXX?
}

func (c *dialedPUDPConn) Read(b []byte) (int, error) {
	for {
		nr, path, err := c.dialedConn.ReadPath(b)
		if err != nil {
			return 0, err
		}
		v := &pudpControllerPacketVisitor{}
		err = pudpParseHeader(b[:nr], v)
		if err != nil {
			continue
		}
		err = c.controller.registerPacket(path, v.identifier, v.pongSequenceNum)
		if err != nil {
			continue
		}
		n := copy(b, v.pld)
		return n, nil
	}
}

func (c *dialedPUDPConn) Close() error {
	c.controller.Close()
	return c.dialedConn.Close()
}

type pudpController struct {
	mutex sync.Mutex

	// client side settings
	maxRace       int
	probeInterval time.Duration
	probeWindow   time.Duration

	paths   []*Path // XXX: active set?
	current *Path

	remoteIdentifier []IfID

	raceSequenceNum uint16
	pingSequenceNum uint16
	pingTime        time.Time

	stop chan struct{}
}

func (c *pudpController) Path() *Path {
	panic("not implemented") // We only call connectedConn.WritePath(), bypassing calls to this func
}

func (c *pudpController) decide() ([]*Path, []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	header := pudpHeaderBuilder{} // TODO use some buffer somewhere?
	paths := []*Path{c.current}
	if c.current == nil {
		// no path selected (yet), we're racing
		paths = c.paths
		if len(paths) > c.maxRace {
			paths = paths[:c.maxRace]
		}
		if len(paths) > 1 {
			header.race(c.raceSequenceNum)
			c.raceSequenceNum++
		}
		// send one first ping during racing
		if (c.pingTime == time.Time{}) {
			c.pingTime = time.Now() // TODO:
			header.ping(c.pingSequenceNum)
		}
		if c.remoteIdentifier == nil {
			header.identify()
		}
	}
	// TODO else, ping if in window?

	header.buf.WriteByte(byte(pudpHeaderPayload))
	return paths, header.buf.Bytes()
}

func (c *pudpController) SetPaths(paths []*Path) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.paths = filterPathsByLastHopInterface(paths, c.remoteIdentifier)
	// TODO reset c.current! Set current again or switch path
}

func (c *pudpController) OnPathDown(pf PathFingerprint, pi PathInterface) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.current != nil &&
		(isInterfaceOnPath(c.current, pi) || pf == c.current.Fingerprint) {
		panic("not implemented") // TODO: Implement
	}
}

func (c *pudpController) Run(udpConn Conn) {
	probeTimer := time.NewTimer(0)
	<-probeTimer.C
	inProbeWindow := false
	for {
		select {
		case <-c.stop:
			break
		case <-probeTimer.C:
			if !inProbeWindow {
				// start probe interval
				probeTimer.Reset(c.probeWindow)
				inProbeWindow = true

			} else {
				// probe window ended
				probeTimer.Reset(c.probeInterval - c.probeWindow)
				inProbeWindow = false
			}
		}
	}
}

func (c *pudpController) Close() error {
	c.stop <- struct{}{}
	return nil
}

func (c *pudpController) registerPacket(path *Path, identifier []IfID,
	pongSequenceNum interface{}) error {

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Register first remote identifier; effectively, pick an anycast instance
	if c.remoteIdentifier == nil && identifier != nil {
		c.remoteIdentifier = identifier
		c.paths = filterPathsByLastHopInterface(c.paths, c.remoteIdentifier)
	} else if c.remoteIdentifier != nil && identifier != nil {
		// if we've previously seen a remote identifier, drop the packet if this
		// does not match (response from a different anycast instance)
		// Simple comparison seems ok here, if it's the same instance there's no reason it should
		// e.g. change the order or the entries.
		if !ifidSliceEqual(c.remoteIdentifier, identifier) {
			return errors.New("unexpected non-matching remote identifier")
		}
	}

	// If we are awaiting the first reply packet during/after racing, pick this
	// path to continue sending on (if it is indeed in the set of available
	// paths...).
	fmt.Println("here")
	if c.current == nil {
		fmt.Println("path.Fingerprint", path.Fingerprint)
		for _, p := range c.paths {
			fmt.Println("p.Fingerprint", p.Fingerprint)
			if p.Fingerprint == path.Fingerprint {
				c.current = p
				break
			}
		}
	}

	if pongSequenceNum != nil {
		c.registerPong(pongSequenceNum.(uint16), path)
	}
	return nil
}

func (c *pudpController) registerPong(seq uint16, path *Path) {
	if c.pingSequenceNum == seq {
		// XXX: we should register the samples already when
		// sending the probe and then update it on the pong. This will give a
		// better view of dead paths when sorting
		stats.RegisterLatency(path.Fingerprint, time.Since(c.pingTime))
	}
}

func filterPathsByLastHopInterface(paths []*Path, interfaces []IfID) []*Path {
	// empty interface list means don't care
	if len(interfaces) == 0 {
		return paths
	}
	filtered := make([]*Path, 0, len(paths))
	for _, p := range paths {
		if p.Metadata != nil || len(p.Metadata.Interfaces) > 0 {
			last := p.Metadata.Interfaces[len(p.Metadata.Interfaces)-1]
			// if last in list, keep it:
			for _, ifid := range interfaces {
				if last.IfID == ifid {
					filtered = append(filtered, p)
					break
				}
			}
		}
	}
	return filtered
}

func ifidSliceEqual(a, b []IfID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type pudpControllerPacketVisitor struct {
	pld             []byte
	identifier      []IfID
	pongSequenceNum interface{} // optional int
}

func (v *pudpControllerPacketVisitor) payload(b []byte) {
	v.pld = b
}

func (v *pudpControllerPacketVisitor) race(seq uint16) {
	// ignore (or error!?)
}

func (v *pudpControllerPacketVisitor) ping(seq uint16) {
	// ignore (or error!?)
}

func (v *pudpControllerPacketVisitor) pong(seq uint16) {
	v.pongSequenceNum = seq
}

func (v *pudpControllerPacketVisitor) identify() {
	// ignore (or error!?)
}

func (v *pudpControllerPacketVisitor) me(ifids []IfID) {
	v.identifier = ifids
}
