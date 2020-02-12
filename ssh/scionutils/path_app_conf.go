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

package scionutils

import (
	"errors"
	"github.com/scionproto/scion/go/lib/pathpol"
)

// PathSelection represents a user-specified path selection mode.
// Arbitrary: arbitrary path selection
// Static: use the first selected path for the whole connection
// RoundRobin: iterate through available paths in a circular fashion
type PathSelection int

// Valid PathSelection values:
const (
	Arbitrary PathSelection = iota
	Static
	RoundRobin
)

// PathSelectionFromString parses a string into a PathSelection.
func PathSelectionFromString(s string) (PathSelection, error) {
	switch s {
	case "arbitrary":
		return Arbitrary, nil
	case "static":
		return Static, nil
	case "round-robin":
		return RoundRobin, nil
	default:
		return 0, errors.New("unknown path selection option")
	}
}

// PathAppConf represents application paths configurations specified by the user using command-line arguments
// policy: SCION path policy
// pathSelection: path selection mode
type PathAppConf struct {
	policy        *pathpol.Policy
	pathSelection PathSelection
}

// NewPathAppConf constructs a PathAppConf.
func NewPathAppConf(policy *pathpol.Policy, pathSelection string) (*PathAppConf, error) {
	ps, err := PathSelectionFromString(pathSelection)
	if err != nil {
		return nil, err
	}
	return &PathAppConf{
		policy:        policy,
		pathSelection: ps,
	}, nil
}

// PathSelection returns the PathSelection in the configuration.
func (c *PathAppConf) PathSelection() PathSelection {
	return c.pathSelection
}

// Policy returns the pathpol.Policy in the configuration.
func (c *PathAppConf) Policy() *pathpol.Policy {
	return c.policy
}
