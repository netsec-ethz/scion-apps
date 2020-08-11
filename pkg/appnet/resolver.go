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
	"errors"
	"fmt"

	"github.com/scionproto/scion/go/lib/snet"
)

// Resolver is the interface to resolve a host name to a SCION host address.
// Currently, this is implemented for reading a hosts file and RAINS
type Resolver interface {
	// Resolve finds an address for the name.
	// Returns a HostNotFoundError if the name was not found, but otherwise no
	// error occurred.
	Resolve(name string) (*snet.SCIONAddress, error)
}

// HostNotFoundError is returned by a Resolver when the name was not found, but
// otherwise no error occurred.
type HostNotFoundError struct {
	Host string
}

func (e *HostNotFoundError) Error() string {
	return fmt.Sprintf("host not found: '%s'", e.Host)
}

// ResolverList represents a list of Resolvers that are processed in sequence
// to return the first match.
type ResolverList []Resolver

func (resolvers ResolverList) Resolve(name string) (*snet.SCIONAddress, error) {

	var errHostNotFound *HostNotFoundError
	for _, resolver := range resolvers {
		if resolver != nil {
			addr, err := resolver.Resolve(name)
			if err == nil {
				return addr, nil
			} else if !errors.As(err, &errHostNotFound) {
				return addr, err
			}
		}
	}
	return nil, &HostNotFoundError{name}
}
