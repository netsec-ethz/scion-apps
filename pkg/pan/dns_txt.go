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

type dnsResolver struct {
	res dnsTXTResolver
}

type dnsTXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

var _ resolver = &dnsResolver{}

const scionAddrTXTTag = "scion="

// Resolve the name via DNS to return one scionAddr or an error.
func (d *dnsResolver) Resolve(ctx context.Context, name string) (saddr scionAddr, err error) {
	addresses, err := d.queryTXTRecord(ctx, name)
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
func (d *dnsResolver) queryTXTRecord(ctx context.Context, host string) (addresses []string, err error) {
	if d.res == nil {
		return addresses, fmt.Errorf("invalid DNS resolver: %v", d.res)
	}
	if !strings.HasSuffix(host, ".") {
		host += "."
	}
	txtRecords, err := d.res.LookupTXT(ctx, host)
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
		if strings.HasPrefix(txt, scionAddrTXTTag) {
			addresses = append(addresses, strings.TrimPrefix(txt, scionAddrTXTTag))
		}
	}
	if len(addresses) == 0 {
		return addresses, HostNotFoundError{Host: host}
	}
	return addresses, nil
}
