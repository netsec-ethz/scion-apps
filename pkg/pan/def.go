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
	"errors"
	"fmt"
	"time"
)

var ErrNoPath = errors.New("no path")

func errNoPathTo(ia IA) error {
	return fmt.Errorf("%w to %s", ErrNoPath, ia)
}

const (
	// pathRefreshMinInterval is the minimum time between two path refreshs
	pathRefreshMinInterval = 10 * time.Second
	// pathRefreshInterval is the refresh interval in case no paths are expiring, i.e. the interval
	// in which new paths are discovered.
	pathRefreshInterval = 5 * time.Minute
	// pathExpiryRefreshLeadTime specifies when a refresh is triggered for a
	// path, relative to its expiry.
	pathExpiryRefreshLeadTime = 2 * time.Minute
	// pathExpiryPruneLeadTime specifies when, relative to its expiry, a path
	// that is no longer returned from a path query is dropped from the cache.
	pathExpiryPruneLeadTime = pathRefreshMinInterval

	pathDownNotificationTimeout = 10 * time.Second

	defaultSelectorMaxReplyPaths = 4

	statsNumLatencySamples = 4
)

// maxTime is the maximum usable time value (https://stackoverflow.com/a/32620397)
var maxTime = time.Unix(1<<63-62135596801, 999999999)
