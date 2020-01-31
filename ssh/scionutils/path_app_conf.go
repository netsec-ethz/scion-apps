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
