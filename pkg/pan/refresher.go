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
	"math/rand"
	"time"
)

type refreshee interface {
	refresh(dst IA, paths []*Path)
}

type refresher struct {
	subscribers     map[IA][]refreshee
	pool            *pathPool
	newSubscription chan bool
}

func makeRefresher(pool *pathPool) refresher {
	return refresher{
		pool:            pool,
		subscribers:     make(map[IA][]refreshee),
		newSubscription: make(chan bool),
	}
}

// subscribe for paths to dst.
func (r *refresher) subscribe(ctx context.Context, dst IA, s refreshee) ([]*Path, error) {
	// BUG: oops, this will not inform subscribers of updated paths. Need to explicily check here
	paths, err := r.pool.paths(ctx, dst)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errNoPathTo(dst)
	}
	subs, ok := r.subscribers[dst]
	r.subscribers[dst] = append(subs, s)
	if !ok {
		r.newSubscription <- (len(r.subscribers) == 1)
	}
	return paths, nil
}

func (r *refresher) unsubscribe(ia IA, s refreshee) {
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
		}
	}
}

func (r *refresher) refresh() {
	now := time.Now()
	// when a refresh is triggered, we batch all
	for dstIA, subscribers := range r.subscribers {
		poolEntry, _ := r.pool.entry(dstIA)
		if r.shouldRefresh(now, poolEntry.earliestExpiry, poolEntry.lastQuery) {
			paths, err := r.pool.queryPaths(context.Background(), dstIA)
			if err != nil {
				// ignore errors here. The idea is that there is probably a lot of time
				// until this manifests as an actual problem to the application (i.e.
				// when the paths actually expire).
				// TODO: check whether there are errors that could be handled, like try to reconnect
				// to sciond or something like that.
				continue
			}
			for _, subscriber := range subscribers {
				subscriber.refresh(dstIA, paths)
			}
		}
	}
}

func (r *refresher) shouldRefresh(now, expiry, lastQuery time.Time) bool {
	earliestAllowedRefresh := lastQuery.Add(pathRefreshMinInterval)
	timeForRefresh := expiry.Add(-pathExpiryRefreshLeadTime)
	return now.After(earliestAllowedRefresh) && now.After(timeForRefresh)
}

func (r *refresher) untilNextRefresh(prevRefresh time.Time) time.Duration {
	return time.Until(r.nextRefresh(prevRefresh))
}

func (r *refresher) nextRefresh(prevRefresh time.Time) time.Time {
	if len(r.subscribers) == 0 {
		return maxTime
	}
	nextRefresh := prevRefresh.Add(pathRefreshInterval)

	expiry := r.pool.earliestPathExpiry()
	randOffset := time.Duration(rand.Intn(10)) * time.Second // avoid everbody refreshing simultaneously
	expiryRefresh := expiry.Add(-pathExpiryRefreshLeadTime + randOffset)

	if expiryRefresh.Before(nextRefresh) {
		nextRefresh = expiryRefresh
	}

	// if there are still paths that expire very soon (or have already expired),
	// we still wait a little bit until the next refresh. Otherwise, failing
	// refresh of an expired path would make us refresh continuously.
	earliestAllowed := prevRefresh.Add(pathRefreshMinInterval)
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
