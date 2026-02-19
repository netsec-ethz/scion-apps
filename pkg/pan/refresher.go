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
	"maps"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/scionproto/scion/pkg/addr"
)

const (
	refreshIntervalForSubscribersWithoutPath = 6 * time.Second
)

type refreshee interface {
	refresh(dst addr.IA, paths []*Path)
}

type refresher struct {
	subscribersMutex sync.Mutex
	subscribers      map[addr.IA][]refreshee
	newSubscription  chan bool
	pool             *PathPool
}

func makeRefresher(pool *PathPool) refresher {
	return refresher{
		pool:            pool,
		subscribers:     make(map[addr.IA][]refreshee),
		newSubscription: make(chan bool),
	}
}

// subscribe for paths to dst.
func (r *refresher) subscribe(ctx context.Context, dst addr.IA, s refreshee) ([]*Path, error) {
	// BUG: oops, this will not inform subscribers of updated paths. Need to explicily check here
	paths, _, err := r.pool.paths(ctx, dst)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errNoPathTo(dst)
	}
	r.subscribersMutex.Lock()
	defer r.subscribersMutex.Unlock()
	subs, ok := r.subscribers[dst]
	r.subscribers[dst] = append(subs, s)
	if !ok {
		r.newSubscription <- (len(r.subscribers) == 1)
	}
	return paths, nil
}

func (r *refresher) unsubscribe(ia addr.IA, s refreshee) {
	r.subscribersMutex.Lock()
	defer r.subscribersMutex.Unlock()

	idx := -1
	subs := r.subscribers[ia]
	for i, v := range subs {
		if s == v {
			idx = i
			break
		}
	}
	if idx >= 0 {
		r.subscribers[ia] = append(subs[:idx], subs[idx+1:]...)
	}
	if len(r.subscribers[ia]) == 0 {
		delete(r.subscribers, ia)
	}
}

func (r *refresher) run() {
	refreshTimer := time.NewTimer(0)
	refreshTimerForSubscribersWithoutPath := time.NewTicker(
		refreshIntervalForSubscribersWithoutPath,
	)
	<-refreshTimer.C
	var prevRefresh time.Time
	for {
		select {
		case first := <-r.newSubscription:
			// first subscriber: we just did a full refresh by fetching the paths for the first time.
			if first {
				prevRefresh = time.Now()
			}
			// just set the timer again:
			// we could be smarter, but why should we
			nextRefresh := r.untilNextRefresh(prevRefresh)
			resetTimer(refreshTimer, nextRefresh)
		case <-refreshTimer.C:
			r.refresh()
			prevRefresh = time.Now()
			nextRefresh := r.untilNextRefresh(prevRefresh)
			refreshTimer.Reset(nextRefresh)
		case <-refreshTimerForSubscribersWithoutPath.C:
			r.refreshSubscribersWithoutPath()
		}
	}
}

func (r *refresher) refresh() {
	r.refreshIAs(r.subscribersToRefresh(false))
}

func (r *refresher) refreshSubscribersWithoutPath() {
	r.refreshIAs(r.subscribersToRefresh(true))
}

func (r *refresher) subscribersToRefresh(onlySubscribersWithoutPath bool) []addr.IA {
	r.subscribersMutex.Lock()
	defer r.subscribersMutex.Unlock()

	keys := slices.Collect(maps.Keys(r.subscribers))
	if !onlySubscribersWithoutPath {
		return keys
	}

	// Filter to only subscribers without cached paths
	result := keys[:0]
	for _, dstIA := range keys {
		if len(r.pool.cachedPaths(dstIA)) == 0 {
			result = append(result, dstIA)
		}
	}
	return result
}

func (r *refresher) refreshIAs(refreshIAs []addr.IA) {
	for _, dstIA := range refreshIAs {
		paths, areFresh, err := r.pool.paths(context.Background(), dstIA)
		if err != nil || !areFresh {
			// ignore errors here. The idea is that there is probably a lot of time
			// until this manifests as an actual problem to the application (i.e.
			// when the paths actually expire).
			// TODO: check whether there are errors that could be handled, like try to reconnect
			// to sciond or something like that.
			continue
		}
		r.subscribersMutex.Lock()
		for _, subscriber := range r.subscribers[dstIA] {
			subscriber.refresh(dstIA, paths)
		}
		r.subscribersMutex.Unlock()
	}
}

func (r *refresher) untilNextRefresh(prevRefresh time.Time) time.Duration {
	return time.Until(r.nextRefresh(prevRefresh))
}

func (r *refresher) nextRefresh(prevRefresh time.Time) time.Time {
	if len(r.subscribers) == 0 {
		return maxTime
	}

	nextRefresh := prevRefresh.Add(r.pool.config.RefreshInterval)

	expiry := r.pool.earliestPathExpiry()
	// avoid everbody refreshing simultaneously
	randOffset := time.Duration(rand.Intn(10)) * time.Second
	expiryRefresh := expiry.Add(-r.pool.config.RefreshLeadTime + randOffset)

	if expiryRefresh.Before(nextRefresh) {
		nextRefresh = expiryRefresh
	}

	// if there are still paths that expire very soon (or have already expired),
	// we still wait a little bit until the next refresh. Otherwise, failing
	// refresh of an expired path would make us refresh continuously.
	earliestAllowed := prevRefresh.Add(r.pool.config.RefreshMinInterval)
	if nextRefresh.Before(earliestAllowed) {
		return earliestAllowed
	}
	return nextRefresh
}

// resetTimer resets the timer, as described in godoc for time.Timer.Reset.
//
// This cannot be done concurrent to other receives from the Timer's channel or
// other calls to the Timer's Stop method.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		// Drain the event channel if not empty
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
