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
	"net"
)

// Conn represents a _dialed_ connection.
type Conn interface {
	net.Conn
	// SetPolicy allows to set the path policy for paths used by Write, at any
	// time.
	SetPolicy(policy Policy)
	// WritePath writes a message to the remote address via the given path.
	// This bypasses the path policy and selector used for Write.
	WritePath(path *Path, b []byte) (int, error)
	// ReadPath reads a message and returns the (return-)path via which the
	// message was received.
	ReadPath(b []byte) (int, *Path, error)
}

func DialUDP(ctx context.Context, local *net.UDPAddr, remote UDPAddr,
	policy Policy, selector Selector) (Conn, error) {

	local, err := defaultLocalAddr(local)
	if err != nil {
		return nil, err
	}

	if selector == nil {
		selector = &DefaultSelector{}
	}

	raw, slocal, err := openBaseUDPConn(ctx, local)
	if err != nil {
		return nil, err
	}
	var subscriber *pathRefreshSubscriber
	if remote.IA != slocal.IA {
		subscriber, err = openPathRefreshSubscriber(ctx, remote, policy, selector)
		if err != nil {
			return nil, err
		}
	}
	return &dialedConn{
		baseUDPConn: baseUDPConn{
			raw: raw,
		},
		local:      slocal,
		remote:     remote,
		subscriber: subscriber,
		Selector:   selector,
	}, nil
}

type dialedConn struct {
	baseUDPConn

	local      UDPAddr
	remote     UDPAddr
	subscriber *pathRefreshSubscriber
	Selector   Selector
}

func (c *dialedConn) SetPolicy(policy Policy) {
	if c.subscriber != nil {
		c.subscriber.setPolicy(policy)
	}
}

func (c *dialedConn) LocalAddr() net.Addr {
	return c.local
}

func (c *dialedConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *dialedConn) Write(b []byte) (int, error) {
	var path *Path
	if c.local.IA != c.remote.IA {
		path = c.Selector.Path()
		if path == nil {
			return 0, errNoPathTo(c.remote.IA)
		}
	}
	return c.baseUDPConn.writeMsg(c.local, c.remote, path, b)
}

func (c *dialedConn) WritePath(path *Path, b []byte) (int, error) {
	return c.baseUDPConn.writeMsg(c.local, c.remote, path, b)
}

func (c *dialedConn) Read(b []byte) (int, error) {
	for {
		n, remote, _, err := c.baseUDPConn.readMsg(b)
		if err != nil {
			return n, err
		}
		if !remote.Equal(c.remote) {
			continue // connected! Ignore spurious packets from wrong source
		}
		return n, err
	}
}

func (c *dialedConn) ReadPath(b []byte) (int, *Path, error) {
	for {
		n, remote, fwPath, err := c.baseUDPConn.readMsg(b)
		if err != nil {
			return n, nil, err
		}
		if !remote.Equal(c.remote) {
			continue // connected! Ignore spurious packets from wrong source
		}
		path, err := reversePathFromForwardingPath(c.remote.IA, c.local.IA, fwPath)
		if err != nil {
			continue // just drop the packet if there is something wrong with the path
		}
		return n, path, nil
	}
}

func (c *dialedConn) Close() error {
	if c.subscriber != nil {
		_ = c.subscriber.Close()
	}
	if c.Selector != nil {
		_ = c.Selector.Close()
	}
	return c.baseUDPConn.Close()
}

// pathRefreshSubscriber is the glue between a connection and the global path
// pool. It gets the paths to dst and sets the filtered path set on the
// target Selector.
type pathRefreshSubscriber struct {
	remote UDPAddr
	policy Policy
	target Selector
}

func openPathRefreshSubscriber(ctx context.Context, remote UDPAddr, policy Policy,
	target Selector) (*pathRefreshSubscriber, error) {

	s := &pathRefreshSubscriber{
		remote: remote,
		policy: policy,
		target: target,
	}
	paths, err := pool.subscribe(ctx, remote.IA, s)
	if err != nil {
		return nil, nil
	}
	s.setFiltered(paths)
	return s, nil
}

func (s *pathRefreshSubscriber) Close() error {
	pool.unsubscribe(s.remote.IA, s)
	return nil
}

func (s *pathRefreshSubscriber) setPolicy(policy Policy) {
	s.policy = policy
	s.setFiltered(pool.cachedPaths(s.remote.IA))
}

func (s *pathRefreshSubscriber) refresh(dst IA, paths []*Path) {
	s.setFiltered(paths)
}

func (s *pathRefreshSubscriber) OnPathDown(pf PathFingerprint, pi PathInterface) {
	s.target.OnPathDown(pf, pi)
}

func (s *pathRefreshSubscriber) setFiltered(paths []*Path) {
	if s.policy != nil {
		paths = s.policy.Filter(paths)
	}
	s.target.SetPaths(s.remote, paths)
}