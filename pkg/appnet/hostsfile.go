// Copyright 2018 ETH Zurich
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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/scionproto/scion/go/lib/snet"
)

type hostsTable map[string]snet.SCIONAddress

// hostsfileResolver is an implementation of the resolver interface, backed
// by an /etc/hosts-like file.
type hostsfileResolver struct {
	path string
}

// Resolve implements
func (r *hostsfileResolver) Resolve(name string) (*snet.SCIONAddress, error) {

	// Note: obviously not perfectly elegant to parse the entire file for
	// every query. However, properly caching this and still always provide
	// fresh results after changes to the hosts file seems like a bigger task and
	// for now that would be overkill.
	table, err := loadHostsFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("error loading %s: %s", r.path, err)
	}
	addr, ok := table[name]
	if !ok {
		return nil, &HostNotFoundError{name}
	}
	return &addr, nil
}

func loadHostsFile(path string) (hostsTable, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		// not existing file treated like an empty file,
		// just return an empty table
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseHostsFile(file)
}

func parseHostsFile(file *os.File) (hostsTable, error) {
	hosts := make(hostsTable)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// ignore comments
		cstart := strings.IndexRune(line, '#')
		if cstart >= 0 {
			line = line[:cstart]
		}

		// cut into fields: address name1 name2 ...
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if addrRegexp.MatchString(fields[0]) {
			addr, err := addrFromString(fields[0])
			if err != nil {
				continue
			}

			// map hostnames to scionAddress
			for _, name := range fields[1:] {
				hosts[name] = addr
			}
		}
	}
	return hosts, scanner.Err()
}
