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
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
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
	return rainsQuery(server, name)
}

func readRainsConfig() (UDPAddr, error) {
	bs, err := ioutil.ReadFile(rainsConfigPath)
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

func rainsQuery(server UDPAddr, hostname string) (scionAddr, error) {
	const (
		ctx     = "."                    // use global context
		qType   = rains.OTScionAddr      // request SCION addresses
		expire  = 5 * time.Minute        // sensible expiry date?
		timeout = 500 * time.Millisecond // timeout for query
	)
	qOpts := []rains.Option{} // no options

	// TODO(matzf): check that this actually works
	// - return error on timeout, network problems, invalid format, ...
	// - return HostNotFoundError error if all went well, but host not found
	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The timeout value has been decreased to counter this behavior until the problem is resolved.
	srv := server.snetUDPAddr()
	reply, err := rains.Query(hostname, ctx, []rains.Type{qType}, qOpts, expire, timeout, srv)
	if err != nil {
		return scionAddr{}, fmt.Errorf("address for host %q not found: %w", hostname, err)
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
