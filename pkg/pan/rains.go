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

//go:build !norains
// +build !norains

package pan

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/snet"
)

const rainsConfigPath = "/etc/scion/rains.cfg"

func init() {
	resolveRains = &rainsResolver{}
}

type rainsResolver struct{}

var _ resolver = &rainsResolver{}

func (r *rainsResolver) Resolve(ctx context.Context, name string) (scionAddr, error) {
	server, err := readRainsConfig()
	if err != nil {
		return scionAddr{}, err
	}
	if server.Port == 0 {
		// nobody to ask, so we won't get a reply
		return scionAddr{}, HostNotFoundError{name}
	}
	return rainsQuery(ctx, server, name)
}

func readRainsConfig() (UDPAddr, error) {
	bs, err := os.ReadFile(rainsConfigPath)
	if os.IsNotExist(err) {
		return UDPAddr{}, nil
	} else if err != nil {
		return UDPAddr{}, fmt.Errorf("error loading %s: %w", rainsConfigPath, err)
	}
	address, err := ParseUDPAddr(strings.TrimSpace(string(bs)))
	if err != nil {
		return UDPAddr{}, fmt.Errorf("error parsing %s, expected SCION UDP address: %w", rainsConfigPath, err)
	}
	return address, nil
}

func rainsQuery(ctx context.Context, server UDPAddr, hostname string) (scionAddr, error) {
	const (
		rainsCtx = "."                    // use global context
		qType    = rains.OTScionAddr      // request SCION addresses
		expire   = 5 * time.Minute        // sensible expiry date?
		timeout  = 500 * time.Millisecond // timeout for query
	)
	qOpts := []rains.Option{} // no options

	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The (default) timeout value has been decreased to counter this behavior until the problem is resolved.
	srv := &snet.UDPAddr{
		IA:   addr.IA(server.IA),
		Host: net.UDPAddrFromAddrPort(netip.AddrPortFrom(server.IP, server.Port)),
	}

	reply, err := rainsQueryChecked(ctx, hostname, rainsCtx, []rains.Type{qType}, qOpts, expire, timeout, srv)
	if err != nil {
		return scionAddr{}, err
	}
	addrStr, ok := reply[qType]
	if !ok {
		return scionAddr{}, &HostNotFoundError{hostname}
	}
	addr, err := parseSCIONAddr(addrStr)
	if err != nil {
		return scionAddr{}, fmt.Errorf("address for host %q invalid: %w", hostname, err)
	}
	return addr, nil
}

func rainsQueryChecked(ctx context.Context, name, rainsCtx string, types []rains.Type, opts []rains.Option,
	expire, timeout time.Duration, addr net.Addr) (res map[rains.Type]string, err error) {

	var contextTimeout time.Duration
	deadline, finite := ctx.Deadline()
	if finite {
		contextTimeout = time.Until(deadline)
		if contextTimeout < 0 {
			return res, context.DeadlineExceeded
		}
	} else {
		contextTimeout = timeout
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		res, err = rains.Query(name, rainsCtx, types, opts, expire, contextTimeout, addr)
	}()
	select {
	case <-ctx.Done():
		return res, ctx.Err()
	case <-done:
	}
	return
}
