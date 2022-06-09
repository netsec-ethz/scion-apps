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
	"fmt"
	"net"
	"strings"
)

type dnsResolver struct{}
var _ resolver = &dnsResolver{}

func (d *dnsResolver) Resolve(ctx context.Context, name string) (saddr scionAddr, err error) {
	addresses, err := queryTXTRecord(ctx, name)
	if err != nil {
		return scionAddr{}, err
	}
	if len(addresses) == 0 {
		return scionAddr{}, HostNotFoundError{Host: name}
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

func queryTXTRecord(ctx context.Context, query string) (address []string, err error) {
	if !strings.HasSuffix(query,".") {
		query += "."
	}
	resolver := net.Resolver{}
	txtRecords, err := resolver.LookupHost(ctx, query)
	if dnsError, ok := err.(*net.DNSError); ok {
		if dnsError.IsNotFound {
			return address, HostNotFoundError{query}
		}
	}
	if err != nil {
		return address, err
	}
	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "scion=") {
			address = append(address, strings.TrimPrefix(txt, "scion="))
		}
	}
	if len(address) == 0 {
		return address, HostNotFoundError{query}
	}
	return address, nil
}
