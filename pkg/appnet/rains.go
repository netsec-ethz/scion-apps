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

package appnet

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/netsec-ethz/rains/pkg/rains"
	"github.com/scionproto/scion/go/lib/snet"
)

const rainsConfigPath = "/etc/scion/rains.cfg"

var rainsServer *snet.UDPAddr // resolver address

func init() {
	// read RAINS server address
	rainsServer = readRainsConfig()
}

func readRainsConfig() *snet.UDPAddr {
	bs, err := ioutil.ReadFile(rainsConfigPath)
	if err != nil {
		return nil
	}
	address, err := snet.ParseUDPAddr(strings.TrimSpace(string(bs)))
	if err != nil {
		return nil
	}
	return address
}

func rainsQuery(hostname string) (snet.SCIONAddress, error) {

	const (
		ctx     = "."                    // use global context
		qType   = rains.OTScionAddr      // request SCION addresses
		expire  = 5 * time.Minute        // sensible expiry date?
		timeout = 500 * time.Millisecond // timeout for query
	)
	qOpts := []rains.Option{} // no options

	if rainsServer == nil {
		return snet.SCIONAddress{}, fmt.Errorf("could not resolve %q, no RAINS server configured", hostname)
	}

	// TODO(chaehni): This call can sometimes cause a timeout even though the server is reachable (see issue #221)
	// The timeout value has been decreased to counter this behavior until the problem is resolved.
	reply, err := rains.Query(hostname, ctx, []rains.Type{qType}, qOpts, expire, timeout, rainsServer)
	if err != nil {
		return snet.SCIONAddress{}, fmt.Errorf("address for host %q not found: %v", hostname, err)
	}
	addr, err := addrFromString(reply[qType])
	if err != nil {
		return snet.SCIONAddress{}, fmt.Errorf("address for host %q invalid: %v", hostname, err)
	}
	return addr, nil
}
