// Copyright 2022 ETH Zurich
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
	"errors"
	"fmt"
	"net"
	"strings"
)

type dnsResolver struct{}

var _ resolver = &dnsResolver{}

// Resolve the name via DNS to return one scionAddr or an error.
func (d *dnsResolver) Resolve(ctx context.Context, name string) (saddr scionAddr, err error) {
	addresses, err := queryTXTRecord(ctx, name)
	if err != nil {
		return scionAddr{}, err
	}
	var perr error
	for _, addr := range addresses {
		saddr, perr = parseSCIONAddr(addr)
		if perr == nil {
			return saddr, nil
		}
	}
	return scionAddr{}, fmt.Errorf("error parsing TXT SCION address records: %w", perr)
}

// queryTXTRecord queries the DNS for DNS TXT record(s) specifying the SCION address(es) for host.
// Returns either at least one address, or else an error, of type HostNotFoundError if no matching record was found.
func queryTXTRecord(ctx context.Context, host string) (addresses []string, err error) {
	if !strings.HasSuffix(host, ".") {
		host += "."
	}
	resolver := net.Resolver{}
	txtRecords, err := resolver.LookupTXT(ctx, host)
	var errDNSError *net.DNSError
	if errors.As(err, &errDNSError) {
		if errDNSError.IsNotFound {
			return addresses, HostNotFoundError{Host: host}
		}
	}
	if err != nil {
		return addresses, err
	}
	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "scion=") {
			addresses = append(addresses, strings.TrimPrefix(txt, "scion="))
		}
	}
	if len(addresses) == 0 {
		return addresses, HostNotFoundError{Host: host}
	}
	return addresses, nil
}
