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
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Open DNS resolver by SWITCH
const switchResolver1 string = "130.59.31.248"
const switchResolver2 string = "130.59.31.251"

func init() {
	resolveDnsTxt = &dnsResolver{}
}

type dnsResolver struct{}

func (d *dnsResolver) Resolve(name string) (scionAddr, error) {
	addresses, err := queryTXTRecord(name)
	if err != nil {
		return scionAddr{}, err
	}
	for _, addr := range addresses {
		scionAddr, err := parseSCIONAddr(addr)
		if err == nil {
			return scionAddr, nil
		}
	}
	return scionAddr{}, fmt.Errorf("error parsing TXT SCION address records")
}

func queryTXTRecord(query string) (address []string, err error) {
	msg := new(dns.Msg)
	if !strings.HasSuffix(query,".") {
		query += "."
	}
	msg.SetQuestion(query, dns.TypeTXT)
	msg.RecursionDesired = true
	// TODO : add / import logic to use configured local resolver preference
	resolver := net.UDPAddr{IP: net.ParseIP(switchResolver1), Port: 53}
	res, err := dns.Exchange(msg, resolver.String())
	if err != nil {
		return address, err
	}
	for _, ans := range res.Answer {
		if txtRecords, ok := ans.(*dns.TXT); ok {
			for _, txt := range txtRecords.Txt {
				if strings.HasPrefix(txt, "scion=") {
					address = append(address, strings.TrimPrefix(txt, "scion="))
				}
			}
		}
	}
	if len(address) == 0 {
		return address, fmt.Errorf("no TXT record with SCION address found")
	}
	return address, nil
}
