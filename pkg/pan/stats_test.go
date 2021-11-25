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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNotifyPathDown(t *testing.T) {
	stats = newPathStatsDB()

	pf := PathFingerprint("x")
	pi := PathInterface{
		IA:   MustParseIA("1-ff00:0:110"),
		IfID: 5,
	}

	// subscribe, signal on channel when notification happened
	subscriber := &dummyPathDownSubscriber{notified: make(chan struct{})}
	stats.subscribe(subscriber)

	stats.NotifyPathDown(pf, pi)

	timeout := time.Second
	select {
	case <-subscriber.notified:
	case <-time.NewTimer(timeout).C:
		assert.FailNow(t, "stats.NotifyPathDown did not notify subscriber before timeout", timeout)
	}

	stats.unsubscribe(subscriber)
	assert.Empty(t, stats.notifier.subscribers)
}

type dummyPathDownSubscriber struct {
	notified chan struct{}
}

func (s *dummyPathDownSubscriber) PathDown(_ PathFingerprint, _ PathInterface) {
	s.notified <- struct{}{}
}

func TestRecordLatency(t *testing.T) {
	stats = newPathStatsDB()

	dst := mustParseSCIONAddr("1-ff00:0:110,192.0.2.1")
	p := PathFingerprint("x")

	// latency values to insert
	values := []time.Duration{1, 2, 3, 4, 5, 6, 7}

	// expected latency entries after each insertion step
	expected := [][]time.Duration{
		{1},
		{2, 1},
		{3, 2, 1},
		{4, 3, 2, 1},
		{5, 4, 3, 2}, // spilled, statsNumLatencySamples == 4
		{6, 5, 4, 3},
		{7, 6, 5, 4},
	}

	for step, v := range values {
		stats.RecordLatency(dst, p, v)

		actualStats := stats.destinations[dst].Latency[p]
		actual := make([]time.Duration, len(actualStats))
		for j, a := range actualStats {
			actual[j] = a.Value
		}
		assert.Equal(t, expected[step], actual)
	}
}

func TestLowestLatency(t *testing.T) {
	dst := mustParseSCIONAddr("1-ff00:0:110,192.0.2.1")

	p0 := &Path{Fingerprint: "p0"}
	p1 := &Path{Fingerprint: "p1"}
	p2 := &Path{Fingerprint: "p2"}
	testPaths := []*Path{p0, p1, p2}

	cases := []struct {
		name string
		// setup sets up the latency and path down notifications in the stats db
		setup func(stats *pathStatsDB)
		// paths to use in the LowestLatency query
		paths []*Path
		// expected index of the path returned by LowestLatency
		expected int
	}{
		{
			name:     "empty",
			setup:    func(stats *pathStatsDB) {},
			paths:    nil,
			expected: -1,
		},
		{
			name:     "default",
			setup:    func(stats *pathStatsDB) {},
			paths:    testPaths,
			expected: 0,
		},
		{
			name: "only path with latency data",
			setup: func(stats *pathStatsDB) {
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
			},
			paths:    testPaths,
			expected: 1,
		},
		{
			name: "only path with latency data is down",
			setup: func(stats *pathStatsDB) {
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
				stats.NotifyPathDown(p1.Fingerprint, PathInterface{})
			},
			paths:    testPaths,
			expected: 0,
		},
		{
			name: "only path with no down notification",
			setup: func(stats *pathStatsDB) {
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
				stats.NotifyPathDown(p1.Fingerprint, PathInterface{})
				stats.NotifyPathDown(p0.Fingerprint, PathInterface{})
			},
			paths:    testPaths,
			expected: 2,
		},
		{
			name: "path with oldest down notification",
			setup: func(stats *pathStatsDB) {
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
				stats.NotifyPathDown(p1.Fingerprint, PathInterface{})
				stats.NotifyPathDown(p0.Fingerprint, PathInterface{})
				stats.NotifyPathDown(p2.Fingerprint, PathInterface{})
			},
			paths:    testPaths,
			expected: 1,
		},
		{
			name: "lowest latency",
			setup: func(stats *pathStatsDB) {
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
				stats.RecordLatency(dst, p2.Fingerprint, 4*time.Millisecond)
			},
			paths:    testPaths,
			expected: 2,
		},
		{
			name: "lowest latency old down notifications",
			setup: func(stats *pathStatsDB) {
				stats.NotifyPathDown(p1.Fingerprint, PathInterface{})
				stats.NotifyPathDown(p2.Fingerprint, PathInterface{})
				stats.RecordLatency(dst, p1.Fingerprint, 10*time.Millisecond)
				stats.RecordLatency(dst, p2.Fingerprint, 4*time.Millisecond)
			},
			paths:    testPaths,
			expected: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stats := newPathStatsDB()
			c.setup(&stats)
			actual := stats.LowestLatency(dst, c.paths)
			assert.Equal(t, c.expected, actual)
		})
	}
}
