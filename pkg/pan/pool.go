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
	"sync"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/log"
)

type PathSource interface {
	Paths(ctx context.Context, dst addr.IA) ([]*Path, error)
}

type PathPoolConfig struct {
	RefreshMinInterval time.Duration
	RefreshInterval    time.Duration
	RefreshLeadTime    time.Duration
	PruneLeadTime      time.Duration
}

func DefaultPathPoolConfig() PathPoolConfig {
	return PathPoolConfig{
		RefreshMinInterval: pathRefreshMinInterval,
		RefreshInterval:    pathRefreshInterval,
		RefreshLeadTime:    pathRefreshLeadTime,
		PruneLeadTime:      pathPruneLeadTime,
	}
}

func NewPathPool(pathSource PathSource, config PathPoolConfig) *PathPool {
	p := &PathPool{
		pathSource: pathSource,
		entries:    make(map[addr.IA]pathPoolDst),
		config:     config,
	}
	p.refresher = makeRefresher(p)
	// note: start refresher, but won't do anything until paths are added to the pool
	go func() {
		defer log.HandlePanic()
		p.refresher.run()
	}()
	return p
}

type PathPool struct {
	pathSource   PathSource
	refresher    refresher
	entriesMutex sync.RWMutex
	entries      map[addr.IA]pathPoolDst
	config       PathPoolConfig
}

// pathPoolDst is path pool entry for one destination IA
type pathPoolDst struct {
	lastQuery      time.Time
	earliestExpiry time.Time
	paths          []*Path
}

type pathPoolSubscriber interface {
	refreshee
	pathDownNotifyee
}

func (p *PathPool) subscribe(
	ctx context.Context,
	dstIA addr.IA,
	s pathPoolSubscriber,
) ([]*Path, error) {
	paths, err := p.refresher.subscribe(ctx, dstIA, s)
	if err != nil {
		return nil, err
	}
	stats.subscribe(s)
	return paths, nil
}

func (p *PathPool) unsubscribe(dstIA addr.IA, s pathPoolSubscriber) {
	p.refresher.unsubscribe(dstIA, s)
	stats.unsubscribe(s)
}

// paths returns paths to dstIA. This _may_ query paths, unless they have recently been queried.
func (p *PathPool) paths(ctx context.Context, dstIA addr.IA) ([]*Path, bool, error) {
	p.entriesMutex.RLock()
	entry, ok := p.entries[dstIA]
	p.entriesMutex.RUnlock()

	if ok && time.Since(entry.lastQuery) < p.config.RefreshMinInterval {
		return append([]*Path{}, entry.paths...), false, nil
	}

	paths, err := p.QueryPaths(ctx, dstIA)
	if err != nil {
		return nil, false, err
	}
	return paths, true, nil
}

// queryPaths returns paths to dstIA. Unconditionally requests paths from sciond.
func (p *PathPool) QueryPaths(ctx context.Context, dstIA addr.IA) ([]*Path, error) {
	paths, err := p.pathSource.Paths(ctx, dstIA)
	if err != nil {
		return nil, err
	}
	p.entriesMutex.Lock()
	defer p.entriesMutex.Unlock()
	entry := p.entries[dstIA]
	entry.update(paths, p.config.PruneLeadTime)
	p.entries[dstIA] = entry
	return append([]*Path{}, paths...), nil
}

// cachedPaths returns paths to dstIA. Always returns the cached paths, never queries paths.
func (p *PathPool) cachedPaths(dstIA addr.IA) []*Path {
	p.entriesMutex.RLock()
	defer p.entriesMutex.RUnlock()
	return append([]*Path{}, p.entries[dstIA].paths...)
}

func (e *pathPoolDst) update(paths []*Path, pathPruneLeadTime time.Duration) {
	now := time.Now()
	expiryDropTime := now.Add(-pathPruneLeadTime)

	// the updated entry includes all new paths.
	// Any non-expired old path not included in the new paths is appended to the
	// back (but in same order)
	newPathSet := make(map[PathFingerprint]struct{}, len(paths))
	for _, p := range paths {
		newPathSet[p.Fingerprint] = struct{}{}
	}
	for _, old := range e.paths {
		if _, ok := newPathSet[old.Fingerprint]; !ok && old.Expiry.After(expiryDropTime) {
			paths = append(paths, old)
		}
	}

	e.lastQuery = now
	e.earliestExpiry = earliestPathExpiry(paths)
	e.paths = paths
}

func (p *PathPool) earliestPathExpiry() time.Time {
	p.entriesMutex.RLock()
	defer p.entriesMutex.RUnlock()
	if len(p.entries) == 0 {
		return time.Time{}
	}
	ret := maxTime
	for _, entry := range p.entries {
		if entry.earliestExpiry.Before(ret) {
			ret = entry.earliestExpiry
		}
	}
	return ret
}

func earliestPathExpiry(paths []*Path) time.Time {
	if len(paths) == 0 {
		return time.Time{}
	}
	ret := maxTime
	for _, p := range paths {
		expiry := p.Expiry
		if expiry.Before(ret) {
			ret = expiry
		}
	}
	return ret
}
