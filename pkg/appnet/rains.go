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

// +build !norains

package appnet

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	"github.com/scionproto/scion/go/lib/snet"
)

const rainsConfigPath = "/etc/scion/rains.cfg"

func init() {
	resolveRains = &rainsResolver{}
}

type rainsResolver struct{}

func (r *rainsResolver) Resolve(name string) (*snet.SCIONAddress, error) {
	server, err := readRainsConfig()
	if err != nil {
		return nil, err
	}
	if server == nil {
		// nobody to ask, so we won't get a reply
		return nil, &HostNotFoundError{name}
	}
	return rainsQuery(server, name)
}

func readRainsConfig() (*snet.UDPAddr, error) {
	bs, err := ioutil.ReadFile(rainsConfigPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error loading %s: %s", rainsConfigPath, err)
	}
	address, err := snet.ParseUDPAddr(strings.TrimSpace(string(bs)))
	if err != nil {
		return nil, fmt.Errorf("error parsing %s, expected SCION UDP address: %s", rainsConfigPath, err)
	}
	return address, nil
}

func rainsQuery(server *snet.UDPAddr, hostname string) (*snet.SCIONAddress, error) {

	const (
		ctx     = "."                    // use global context
		qType   = rains.OTScionAddr      // request SCION addresses
		expire  = 5 * time.Minute        // sensible expiry date?
		timeout = 500 * time.Millisecond // timeout for query
	)
	qOpts := []rains.Option{} // no options

	// TODO(matzf): check that this behaves as expected:
	// - return error on timeout, network problems, invalid format, ...
	// - return HostNotFoundError error if all went well, but host not found
	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The timeout value has been decreased to counter this behavior until the problem is resolved.
	reply, err := rains.Query(hostname, ctx, []rains.Type{qType}, qOpts, expire, timeout, server)
	if err != nil {
		return nil, fmt.Errorf("address for host %q not found: %v", hostname, err)
	}
	addrStr, ok := reply[qType]
	if !ok {
		return nil, &HostNotFoundError{hostname}
	}
	addr, err := addrFromString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("address for host %q invalid: %v", hostname, err)
	}
	return &addr, nil
}
