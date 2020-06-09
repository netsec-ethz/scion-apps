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

	"github.com/scionproto/scion/go/lib/snet"
)

const lowestRTTReevaluateInterval = 1 * time.Second

type Policy interface {
	// Select lets the Policy choose a path based on the information collected in
	// the path info.
	// The policy returns the index of the selected path.
	// The second return value specifies a time at which this choice should be re-evaluated.
	// Note: if the selected path is revoked or expires, the policy may be re-evaluated earlier.
	// TODO(matzf): collect overall sessions statistics and pass to policy?
	Select(paths []*pathInfo) (int, time.Time)
}

// lowestRTT is a very simple Policy that selects the path with lowest measured
// RTT. In the absence of measured RTTs, it will return the path with fewest
// hops.
type lowestRTT struct {
}

func (*lowestRTT) Select(paths []*pathInfo) (int, time.Time) {
	best := 0
	for i := 1; i < len(paths); i++ {
		if paths[best].revoked && !paths[i].revoked || // prefer non-revoked path
			paths[i].rtt < paths[best].rtt ||
			numHops(paths[i].path) < numHops(paths[best].path) {

			best = i
		}
	}
	return best, time.Now().Add(lowestRTTReevaluateInterval)
}

// maxBW is a Policy that
type maxBW struct {
}

func numHops(path snet.Path) int {
	return len(path.Interfaces())
}
