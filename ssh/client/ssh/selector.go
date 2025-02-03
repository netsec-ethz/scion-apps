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

package ssh

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

var (
	AvailablePathSelectors = []string{"default", "ping", "round-robin", "random"}
)

func selectorByName(name string) (pan.Selector, error) {
	switch name {
	case "default":
		return nil, nil
	case "ping":
		// Set Pinging Selector with active probing on four paths
		// Note: this would ideally be configurable
		selector := &pan.PingingSelector{
			Interval: 2 * time.Second,
			Timeout:  time.Second,
		}
		selector.SetActive(4)
		return selector, nil
	case "round-robin":
		return &roundRobinSelector{}, nil
	case "random":
		return &randomSelector{}, nil
	default:
		return nil, errors.New("unknown path selection option")
	}
}

type roundRobinSelector struct {
	mutex   sync.Mutex
	paths   []*pan.Path
	current int
}

func (s *roundRobinSelector) Path(_ context.Context) *pan.Path {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.paths) == 0 {
		return nil
	}
	p := s.paths[s.current]
	s.current = (s.current + 1) % len(s.paths)
	return p
}

func (s *roundRobinSelector) Initialize(local, remote pan.UDPAddr, paths []*pan.Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.paths = paths
	s.current = 0
}

func (s *roundRobinSelector) Refresh(paths []*pan.Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.paths = paths
	s.current = 0 // just start at the beginning again
}

func (s *roundRobinSelector) PathDown(pf pan.PathFingerprint, pi pan.PathInterface) {
	// ignore dead paths, just send on these anyway
}

func (s *roundRobinSelector) Close() error {
	return nil
}

type randomSelector struct {
	mutex sync.Mutex
	paths []*pan.Path
}

func (s *randomSelector) Path(_ context.Context) *pan.Path {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.paths) == 0 {
		return nil
	}
	r := rand.Intn(len(s.paths))
	p := s.paths[r]
	return p
}

func (s *randomSelector) Initialize(local, remote pan.UDPAddr, paths []*pan.Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.paths = paths
}

func (s *randomSelector) Refresh(paths []*pan.Path) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.paths = paths
}

func (s *randomSelector) PathDown(pf pan.PathFingerprint, pi pan.PathInterface) {
	// ignore dead paths, just send on these anyway
}

func (s *randomSelector) Close() error {
	return nil
}
