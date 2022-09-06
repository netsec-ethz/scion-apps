// Copyright 2022 Thorben Krüger
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

//go:build !nodns
// +build !nodns

package pan

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

type dnsResolver struct {
	serveraddr string
}

func DNSLoadResolvConf(resolvconf string) resolver {
	return &dnsResolver{"127.0.0.1:53"}
}

func (r *dnsResolver) Resolve(name string) (addr scionAddr, err error) {

	var reply *dns.Msg
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn(name), dns.TypeTXT)

	reply, err = dns.Exchange(query, r.serveraddr)
	if err != nil {
		return
	}

	err = fmt.Errorf("received reply with no answers")
	for _, answer := range reply.Answer {
		if rr, ok := answer.(*dns.TXT); ok {
			record := strings.Join(rr.Txt, "")
			if strings.HasPrefix(record, "scion=") {
				tokens := strings.Split(record, "=")
				addr, err = parseSCIONAddr(tokens[1])
				return
			}
		}
		err = fmt.Errorf("received reply without TXT RR")
	}
	return
}
