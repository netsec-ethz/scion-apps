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

package pan

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type hostsTable map[string]scionAddr

var cachedHostsTable struct {
	mapping hostsTable
	updated time.Time
	sync.Mutex
}

// hostsfileResolver is an implementation of the resolver interface, backed
// by an /etc/hosts-like file.
type hostsfileResolver struct {
	path string
}

var _ resolver = &hostsfileResolver{}

func (r *hostsfileResolver) Resolve(ctx context.Context, name string) (scionAddr, error) {
	cachedHostsTable.Lock()
	defer cachedHostsTable.Unlock()
	table, err := loadHostsFile(r.path)
	if err != nil {
		return scionAddr{}, fmt.Errorf("error loading %s: %w", r.path, err)
	}
	addr, ok := table[name]
	if !ok {
		return scionAddr{}, HostNotFoundError{name}
	}
	return addr, nil
}

// loadHostsFile provides a cached copy of the hostsTable or loads and parses the host file at path if it was modified
// since the last update.
func loadHostsFile(path string) (hostsTable, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		// not existing file treated like an empty file,
		// just return an empty table
		return hostsTable(nil), nil
	}
	if err != nil {
		return hostsTable(nil), err
	}
	if stat.ModTime().Before(cachedHostsTable.updated) {
		return cachedHostsTable.mapping, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return hostsTable(nil), err
	}
	defer file.Close()
	mapping, err := parseHostsFile(file)
	if err != nil {
		return hostsTable(nil), err
	}
	cachedHostsTable.mapping = mapping
	cachedHostsTable.updated = time.Now()
	return cachedHostsTable.mapping, nil
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
		addr, err := parseSCIONAddr(fields[0])
		if err != nil {
			continue
		}

		// map hostnames to scionAddress
		for _, name := range fields[1:] {
			hosts[name] = addr
		}
	}
	return hosts, scanner.Err()
}
