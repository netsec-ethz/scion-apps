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
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scionproto/scion/pkg/addr"

	"github.com/netsec-ethz/scion-apps/pkg/pan/internal/ping"
)

// Selector controls the path used by a single **dialed** socket. Stateful.
type Selector interface {
	// Path selects the path for the next packet.
	// Invoked for each packet sent with Write.
	Path(remote UDPAddr) (*Path, error)

	NewRemote(remote UDPAddr) error
	// inform the selector, that it is now responsible for another address
	// which must be in the same AS as any previous addresses
	// this gives more complex selectors (i.e. Pinging) the chance to
	// ping more than one scion address

	// get the IA for which this selector provides paths
	GetIA() IA

	// Initialize the selector for a connection with the initial list of paths,
	// filtered/ordered by the Policy.
	// Invoked once during the creation of a Conn.
	Initialize(local, remote UDPAddr, paths []*Path)
	// Refresh updates the paths. This is called whenever the Policy is changed or
	// when paths were about to expire and are refreshed from the SCION daemon.
	// The set and order of paths may differ from previous invocations.
	Refresh([]*Path)
	// PathDown is called whenever an SCMP down notification is received on any
	// connection so that the selector can adapt its path choice. The down
	// notification may be for unrelated paths not used by this selector.
	PathDown(PathFingerprint, PathInterface)
	Close() error
}

// DefaultSelector is a Selector for a single dialed socket.
// This will keep using the current path, starting with the first path chosen
// by the policy, as long possible.
// Faults are detected passively via SCMP down notifications; whenever such
// a down notification affects the current path, the DefaultSelector will
// switch to the first path (in the order defined by the policy) that is not
// affected by down notifications.
type DefaultSelector struct {
	mutex     sync.Mutex
	paths     []*Path
	current   int
	remote_ia IA
}

func (s *DefaultSelector) GetIA() IA {
	return s.remote_ia
}

func (s *DefaultSelector) NewRemote(remote UDPAddr) error {
	if remote.IA != s.remote_ia {
		return errors.New("address must be inside the AS, which the selector was initialized with")
	}
	return nil
}

func NewDefaultSelector() *DefaultSelector {
	return &DefaultSelector{}
}

func (s *DefaultSelector) Path(remote UDPAddr) (*Path, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.paths) == 0 {
		return nil, errors.New("DefaultPathSelector initialized with empty path list and never refreshed")
	}
	return s.paths[s.current], nil
}

func (s *DefaultSelector) Initialize(local, remote UDPAddr, paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.remote_ia = remote.IA

	s.paths = paths
	s.current = 0
}

func (s *DefaultSelector) Refresh(paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newcurrent := 0
	if len(s.paths) > 0 {
		currentFingerprint := s.paths[s.current].Fingerprint
		for i, p := range paths {
			if p.Fingerprint == currentFingerprint {
				newcurrent = i
				break
			}
		}
	}
	s.paths = paths
	s.current = newcurrent
}

func (s *DefaultSelector) PathDown(pf PathFingerprint, pi PathInterface) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if current := s.paths[s.current]; isInterfaceOnPath(current, pi) || pf == current.Fingerprint {
		fmt.Println("down:", s.current, len(s.paths))
		better := stats.FirstMoreAlive(current, s.paths)
		if better >= 0 {
			// Try next path. Note that this will keep cycling if we get down notifications
			s.current = better
			fmt.Println("failover:", s.current, len(s.paths))
		}
	}
}

func (s *DefaultSelector) Close() error {
	return nil
}

/*
selects the path to each of set of remote hosts
in the same remote AS by periodically pinging them
*/
type PingingSelector struct {
	// Interval for pinging. Must be positive.
	Interval time.Duration
	// Timeout for the individual pings. Must be positive and less than Interval.
	Timeout time.Duration

	mutex   sync.Mutex
	paths   []*Path
	current map[scionAddr]int
	local   scionAddr
	remotes []scionAddr

	numActive    int64
	pingerCtx    context.Context
	pingerCancel context.CancelFunc
	pinger       *ping.Pinger
}

func (s *PingingSelector) GetIA() IA {
	return s.remotes[0].IA
}

func (s *PingingSelector) NewRemote(remote UDPAddr) error {
	if remote.IA != s.remotes[0].IA {
		return errors.New("path selection domain for selectors can only contain addresses inside same AS!")
	}
	s.remotes = append(s.remotes, scionAddr{IA: remote.IA, IP: remote.IP})
	return nil
}

// SetActive enables active pinging on at most numActive paths.
func (s *PingingSelector) SetActive(numActive int) {
	s.ensureRunning()
	atomic.SwapInt64(&s.numActive, int64(numActive))
}

func (s *PingingSelector) Path(remote UDPAddr) (*Path, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.paths) == 0 {
		return nil, errors.New("PingingSelector initialized with empty path list and never refreshed")
	}
	return s.paths[s.current[scionAddr{IA: remote.IA, IP: remote.IP}]], nil
}

func (s *PingingSelector) Initialize(local, remote UDPAddr, paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.local = local.scionAddr()
	s.remotes = append(s.remotes, remote.scionAddr())
	s.paths = paths
	for _, rem := range s.remotes {
		s.current[rem] = stats.LowestLatency(rem, s.paths)
	}
	//s.current = stats.LowestLatency(s.remote, s.paths)
}

func (s *PingingSelector) Refresh(paths []*Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.paths = paths
	// s.current = stats.LowestLatency(s.remote, s.paths)
	for _, rem := range s.remotes {
		s.current[rem] = stats.LowestLatency(rem, s.paths)
	}
}

func (s *PingingSelector) PathDown(pf PathFingerprint, pi PathInterface) {
	s.reselectPaths()
}

func (s *PingingSelector) reselectPaths() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// s.current = stats.LowestLatency(s.remote, s.paths)
	for _, rem := range s.remotes {
		s.current[rem] = stats.LowestLatency(rem, s.paths)
	}
}

func (s *PingingSelector) ensureRunning() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.local.IA == s.remotes[0].IA {
		return
	}
	if s.pinger != nil {
		return
	}
	s.pingerCtx, s.pingerCancel = context.WithCancel(context.Background())
	local := s.local.snetUDPAddr()
	pinger, err := ping.NewPinger(s.pingerCtx, host().dispatcher, local)
	if err != nil {
		return
	}
	s.pinger = pinger
	go s.pinger.Drain(s.pingerCtx)
	go s.run()
}

type probeKey struct {
	pf   PathFingerprint
	addr scionAddr
}

func (s *PingingSelector) run() {
	pingTicker := time.NewTicker(s.Interval)
	pingTimeout := time.NewTimer(0)
	if !pingTimeout.Stop() {
		<-pingTimeout.C // drain initial timer event
	}

	var sequenceNo uint16
	replyPending := make(map[probeKey]struct{})

	for {
		select {
		case <-s.pingerCtx.Done():
			return
		case <-pingTicker.C:
			numActive := int(atomic.LoadInt64(&s.numActive))
			if numActive > len(s.paths) {
				numActive = len(s.paths)
			}
			if numActive == 0 {
				continue
			}

			activePaths := s.paths[:numActive]
			for _, p := range activePaths {
				for _, rem := range s.remotes {
					replyPending[probeKey{p.Fingerprint, rem}] = struct{}{}
				}
			}
			sequenceNo++
			s.sendPings(activePaths, sequenceNo)
			resetTimer(pingTimeout, s.Timeout)
		case r := <-s.pinger.Replies:
			s.handlePingReply(r, replyPending, sequenceNo)
			if len(replyPending) == 0 {
				pingTimeout.Stop()
				s.reselectPaths()
			}
		case <-pingTimeout.C:
			if len(replyPending) == 0 {
				continue // already handled above
			}
			for probe := range replyPending {
				stats.RecordLatency(probe.addr, probe.pf, s.Timeout)
				delete(replyPending, probe)
			}
			s.reselectPaths()
		}
	}
}

func (s *PingingSelector) sendPings(paths []*Path, sequenceNo uint16) {
	for _, p := range paths {
		for _, rem := range s.remotes {
			remote := rem.snetUDPAddr()
			remote.Path = p.ForwardingPath.dataplanePath
			remote.NextHop = net.UDPAddrFromAddrPort(p.ForwardingPath.underlay)
			err := s.pinger.Send(s.pingerCtx, remote, sequenceNo, 16)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (s *PingingSelector) handlePingReply(reply ping.Reply,
	expectedReplies map[probeKey]struct{},
	expectedSequenceNo uint16) {
	if reply.Error != nil {
		// handle NotifyPathDown.
		// The Pinger is not using the normal scmp handler in raw.go, so we have to
		// reimplement this here.
		pf, err := reversePathFingerprint(reply.Path)
		if err != nil {
			return
		}
		switch e := reply.Error.(type) { //nolint:errorlint
		case ping.InternalConnectivityDownError:
			pi := PathInterface{
				IA:   IA(e.IA),
				IfID: IfID(e.Egress),
			}
			stats.NotifyPathDown(pf, pi)
		case ping.ExternalInterfaceDownError:
			pi := PathInterface{
				IA:   IA(e.IA),
				IfID: IfID(e.Interface),
			}
			stats.NotifyPathDown(pf, pi)
		}
		return
	}

	if reply.Source.Host.Type() != addr.HostTypeIP {
		return // ignore replies from non-IP addresses
	}
	src := scionAddr{
		IA: IA(reply.Source.IA),
		IP: reply.Source.Host.IP(),
	}
	if !slices.Contains(s.remotes, src) || reply.Reply.SeqNumber != expectedSequenceNo {
		return
	}
	pf, err := reversePathFingerprint(reply.Path)
	if err != nil {
		return
	}
	if _, expected := expectedReplies[probeKey{pf, src}]; !expected {
		return
	}
	stats.RecordLatency(src, pf, reply.RTT())
	delete(expectedReplies, probeKey{pf, src})
}

func (s *PingingSelector) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.pinger == nil {
		return nil
	}
	s.pingerCancel()
	return s.pinger.Close()
}
