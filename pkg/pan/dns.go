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
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

const resolvConfPath = "/etc/scion/resolv.conf"

func init() {
	resolveDNS = &dnsResolver{resolvConfPath}
}

type dnsResolver struct {
	filename string
}

func (r *dnsResolver) Resolve(name string) (addr scionAddr, err error) {
	servers := getServersfromResolvConf(r.filename)
	var reply *dns.Msg
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn(name), dns.TypeTXT)

	for _, server := range servers {
		reply, err = dns.Exchange(query, server)
		if err != nil {
			continue
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
	}
	return
}

// See man 5 resolv.conf on a Linux machine.
func getServersfromResolvConf(filename string) (nameservers []string) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return
	}

	s := bufio.NewScanner(file)

	for s.Scan() {
		line := string(s.Bytes())
		if len(line) > 0 && (line[0] == ';' || line[0] == '#') {
			// comment.
			continue
		}
		f := regexp.MustCompile(`[ \r\t\n]+`).Split(line, -1)
		if len(f) < 1 {
			continue
		}

		switch f[0] {
		case "nameserver": // add one name server
			if len(f) > 1 {
				nameservers = append(nameservers, f[1]+":53")
			}
		case "domain": // set search path to just this domain
			// not implemented
		case "search": // set search path to given servers
			// not implemented
		case "options": // magic options
			for _, s := range f[1:] {
				switch {
				case strings.HasPrefix(s, "ndots:"):
					// not implemented
				case strings.HasPrefix(s, "timeout:"):
					// not implemented
				case strings.HasPrefix(s, "attempts:"):
					// not implemented
				case s == "rotate":
					// not implemented
				case s == "single-request" || s == "single-request-reopen":
					// not implemented
				case s == "use-vc" || s == "usevc" || s == "tcp":
					// not implemented
				default:
					// really not implemented
				}
			}
		case "lookup":
			// not implemented
		default:
			// really not implemented
		}
	}
	return
}
