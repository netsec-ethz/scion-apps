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
	"net"
	"sync"
	"time"
)

var stats pathStatsDB

func init() {
	stats = newPathStatsDB()
}

type PathStats struct {
	// Was notified down at the recorded time (0 for never notified down)
	IsNotifiedDown time.Time
}

type PathInterfaceStats struct {
	// Was notified down at the recorded time (0 for never notified down)
	IsNotifiedDown time.Time
}

// scionAddrKey is effectively a host address (IA,IP), just hashable.
type scionAddrKey struct {
	ia IA
	ip [16]byte
}

type DestinationStats struct {
	Latency map[PathFingerprint]StatsLatencySamples
}

type StatsLatencySamples []StatsLatencySample

type StatsLatencySample struct {
	Time  time.Time
	Value time.Duration
}

type pathDownNotifyee interface {
	OnPathDown(PathFingerprint, PathInterface)
}

type pathStatsDB struct {
	mutex sync.RWMutex
	// TODO: this needs a fixed/max capacity and least-recently-used spill over
	// Possibly use separate, explicitly controlled table for paths in dialed connections.
	paths map[PathFingerprint]PathStats
	// TODO: this should rather be "link" or "hop" stats, i.e. identified by two
	// consecutive (unordered?) interface IDs.
	interfaces map[PathInterface]PathInterfaceStats
	// TODO: cleanup of this map on connection end, reference counted handle?
	destinations map[scionAddrKey]DestinationStats

	subscribers []pathDownNotifyee
}

func newPathStatsDB() pathStatsDB {
	return pathStatsDB{
		paths:        make(map[PathFingerprint]PathStats),
		interfaces:   make(map[PathInterface]PathInterfaceStats),
		destinations: make(map[scionAddrKey]DestinationStats),
	}
}

func (s *pathStatsDB) RecordLatency(ia IA, ip net.IP, p PathFingerprint, latency time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	dstK := makeSCIONAddrKey(ia, ip)
	dstStats := s.destinations[dstK]
	if dstStats.Latency == nil {
		dstStats.Latency = make(map[PathFingerprint]StatsLatencySamples)
	}
	dstStats.Latency[p] = dstStats.Latency[p].insert(latency)
	s.destinations[dstK] = dstStats
}

// LowestLatency returns the index of the path with lowest recorded latency.
// In case of ties, lower index paths are preferred.
// Path liveness is taken into account; latency records not younger than a
// recorded down notification for the corresponding path are ignored. Paths
// with no recorded latency value, or with down notifications, are treated with
// least priority.
func (s *pathStatsDB) LowestLatency(ia IA, ip net.IP, paths []*Path) int {
	if len(paths) == 0 {
		return -1
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	dstK := makeSCIONAddrKey(ia, ip)
	dstStats := s.destinations[dstK]

	best := -1
	bestDown := maxTime
	bestLatency := maxDuration
	for i, p := range paths {
		down := s.newestDownNotification(p)
		latencyStats := dstStats.Latency[p.Fingerprint]
		latency := maxDuration
		if len(latencyStats) > 0 && latencyStats[0].Time.After(down) {
			latency = latencyStats[0].Value
		}
		if latency < bestLatency {
			best = i
			bestDown = down
			bestLatency = latency
		} else if latency == bestLatency && down.Before(bestDown) {
			best = i
			bestDown = down
		}
	}
	return best
}

func makeSCIONAddrKey(ia IA, ip net.IP) scionAddrKey {
	k := scionAddrKey{ia: ia}
	copy(k.ip[:], ip[:])
	return k
}

// FirstMoreAlive returns the index of the first path in paths that is strictly "more
// alive" than p, or -1 if there is none.
// A path is considered to be more alive if it does not contain any of p's interfaces that
// are considered down
func (s *pathStatsDB) FirstMoreAlive(p *Path, paths []*Path) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for i, pc := range paths {
		if s.IsMoreAlive(pc, p) {
			return i
		}
	}
	return -1
}

// IsMoreAlive checks if a is strictly "less down" / "more alive" than b.
// Returns true if a does not have any recent down notifications and b does, or
// (more generally) if all down notifications for a are strictly older
// than any down notification for b.
func (s *pathStatsDB) IsMoreAlive(a, b *Path) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	newestA := s.newestDownNotification(a)
	oldestB := s.oldestDownNotification(b)
	return newestA.Before(oldestB.Add(-pathDownNotificationTimeout)) // XXX: what is this value, what does it mean?
}

// newestDownNotification returns the time of the newest relevant down
// notification for path p.
func (s *pathStatsDB) newestDownNotification(p *Path) time.Time {
	newest := s.paths[p.Fingerprint].IsNotifiedDown
	if p.Metadata != nil {
		for _, pi := range p.Metadata.Interfaces {
			if v, ok := s.interfaces[pi]; ok {
				if v.IsNotifiedDown.After(newest) {
					newest = v.IsNotifiedDown
				}
			}
		}
	}
	return newest
}

// oldestDownNotification returns the time of the oldest relevant down
// notification for path p. Returns 0 if no relevant down notifications were
// recorded.
func (s *pathStatsDB) oldestDownNotification(p *Path) time.Time {
	t0 := time.Time{}
	oldest := s.paths[p.Fingerprint].IsNotifiedDown
	if p.Metadata != nil {
		for _, pi := range p.Metadata.Interfaces {
			if v, ok := s.interfaces[pi]; ok && v.IsNotifiedDown != t0 {
				if oldest == t0 || v.IsNotifiedDown.Before(oldest) {
					oldest = v.IsNotifiedDown
				}
			}
		}
	}
	return oldest
}

func (s *pathStatsDB) subscribe(subscriber pathDownNotifyee) {
	s.subscribers = append(s.subscribers, subscriber)
}

func (s *pathStatsDB) unsubscribe(subscriber pathDownNotifyee) {
	idx := -1
	for i, v := range s.subscribers {
		if subscriber == v {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.subscribers = append(s.subscribers[:idx], s.subscribers[idx+1:]...)
	}
}

func (s *pathStatsDB) NotifyPathDown(pf PathFingerprint, pi PathInterface) {
	s.recordPathDown(pf, pi)
	for _, subscriber := range s.subscribers {
		subscriber.OnPathDown(pf, pi)
	}
}

func (s *pathStatsDB) recordPathDown(pf PathFingerprint, pi PathInterface) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	now := time.Now()
	ps := s.paths[pf]
	ps.IsNotifiedDown = now
	s.paths[pf] = ps

	pis := s.interfaces[pi]
	pis.IsNotifiedDown = now
	s.interfaces[pi] = pis
}

func (s StatsLatencySamples) insert(latency time.Duration) StatsLatencySamples {
	if len(s) < statsNumLatencySamples {
		s = append(s, StatsLatencySample{})
	}
	n := len(s)
	copy(s[1:n], s[0:n-1])
	s[0] = StatsLatencySample{
		Time:  time.Now(),
		Value: latency,
	}
	return s
}
