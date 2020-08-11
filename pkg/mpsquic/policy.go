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
	"time"
)

const lowestRTTReevaluateInterval = 1 * time.Second
const rttDiffThreshold = 5 * time.Millisecond

type Policy interface {
	// Select lets the Policy choose a path based on the information collected in
	// the path info.
	// The policy returns the index of the selected path.
	// The second return value specifies a time at which this choice should be re-evaluated.
	// Note: if the selected path is revoked or expires, the policy may be re-evaluated earlier.
	// TODO(matzf): collect overall sessions statistics and pass to policy?
	Select(active int, paths []*pathInfo) (int, time.Time)
}

// lowestRTT is a very simple policy that selects the path with lowest measured
// RTT (with some threshold, preferring the active path). In the absence of
// measured RTTs, this will return the currently active path.
type lowestRTT struct {
}

func (p *lowestRTT) Select(active int, paths []*pathInfo) (int, time.Time) {
	best := active
	for i := 0; i < len(paths); i++ {
		if i == best {
			continue
		}
		if p.better(paths[i], paths[best]) {
			best = i
		}
	}
	return best, time.Now().Add(lowestRTTReevaluateInterval)
}

// better checks whether a is better than b under the lowestRTT policy
func (*lowestRTT) better(a, b *pathInfo) bool {
	return !a.revoked && b.revoked || // prefer non-revoked,
		a.rtt < b.rtt-rttDiffThreshold //  prefer lower RTT
}
