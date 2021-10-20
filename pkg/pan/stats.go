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

type DestinationStats struct {
	Latency map[PathFingerprint]StatsLatencySamples
}

type StatsLatencySamples []StatsLatencySample

type StatsLatencySample struct {
	Time  time.Time
	Value time.Duration
}

type pathDownNotifyee interface {
	PathDown(PathFingerprint, PathInterface)
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
	destinations map[scionAddr]DestinationStats

	notifier pathDownNotifier
}

func newPathStatsDB() pathStatsDB {
	return pathStatsDB{
		paths:        make(map[PathFingerprint]PathStats),
		interfaces:   make(map[PathInterface]PathInterfaceStats),
		destinations: make(map[scionAddr]DestinationStats),
	}
}

func (s *pathStatsDB) RecordLatency(dst scionAddr, p PathFingerprint, latency time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	dstStats := s.destinations[dst]
	if dstStats.Latency == nil {
		dstStats.Latency = make(map[PathFingerprint]StatsLatencySamples)
	}
	dstStats.Latency[p] = dstStats.Latency[p].insert(latency)
	s.destinations[dst] = dstStats
}

// LowestLatency returns the index of the path with lowest recorded latency.
// In case of ties, lower index paths are preferred.
// Path liveness is taken into account; latency records not younger than a
// recorded down notification for the corresponding path are ignored. Paths
// with no recorded latency value, or with down notifications, are treated with
// least priority.
func (s *pathStatsDB) LowestLatency(dst scionAddr, paths []*Path) int {
	if len(paths) == 0 {
		return -1
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	dstStats := s.destinations[dst]

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
	s.notifier.subscribe(subscriber)
}

func (s *pathStatsDB) unsubscribe(subscriber pathDownNotifyee) {
	s.notifier.unsubscribe(subscriber)
}

func (s *pathStatsDB) NotifyPathDown(pf PathFingerprint, pi PathInterface) {
	s.recordPathDown(pf, pi)
	s.notifier.notifyAsync(pf, pi)
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

type pathDownNotifier struct {
	mutex         sync.Mutex
	subscribers   []pathDownNotifyee
	runOnce       sync.Once
	notifications chan<- pathDownNotification
}

func (n *pathDownNotifier) subscribe(subscriber pathDownNotifyee) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.subscribers = append(n.subscribers, subscriber)
}

func (n *pathDownNotifier) unsubscribe(subscriber pathDownNotifyee) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	idx := -1
	for i, v := range n.subscribers {
		if subscriber == v {
			idx = i
			break
		}
	}
	if idx >= 0 {
		n.subscribers = append(n.subscribers[:idx], n.subscribers[idx+1:]...)
	}
}

func (n *pathDownNotifier) run() {
	notifications := make(chan pathDownNotification, pathDownNotificationChannelCapacity)
	n.notifications = notifications

	go func() {
		for notification := range notifications {
			n.notify(notification.Fingerprint, notification.Interface)
		}
	}()
}

func (n *pathDownNotifier) notifyAsync(pf PathFingerprint, pi PathInterface) {
	n.runOnce.Do(n.run)
	n.notifications <- pathDownNotification{Fingerprint: pf, Interface: pi}
}

func (n *pathDownNotifier) notify(pf PathFingerprint, pi PathInterface) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, s := range n.subscribers {
		s.PathDown(pf, pi)
	}
}

type pathDownNotification struct {
	Fingerprint PathFingerprint
	Interface   PathInterface
}
