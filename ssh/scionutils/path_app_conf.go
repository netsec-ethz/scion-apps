package scionutils

import (
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"net"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/snet"
)

// PathSelection is an enum-like struct which serves as a convenient representation for user-specified path selection mode
// Arbitrary: arbitrary path selection
// Static: use the first selected path for the whole connection
// RoundRobin: iterate through available paths in a circular fashion
// Random: randomized path selection, currently not supported
type PathSelection int

// Valid PathSelection values:
const (
	Arbitrary PathSelection = iota
	Static
	RoundRobin
)

func PathSelectionFromString(s string) (PathSelection, error) {
	switch s {
	case "arbitrary":
		return Arbitrary, nil
	case "static":
		return Static, nil
	case "round-robin":
		return RoundRobin, nil
	default:
		return 0, common.NewBasicError("Unknown path selection option", nil)
	}
}

// PathAppConf represents application paths configurations specified by the user using command-line arguments
// policy: SCION path policy
// pathSelection: path selection mode
type PathAppConf struct {
	policy        *pathpol.Policy
	pathSelection PathSelection
}

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

func (c *PathAppConf) PathSelection() PathSelection {
	return c.pathSelection
}

func (c *PathAppConf) Policy() *pathpol.Policy {
	return c.policy
}

func (c *PathAppConf) PolicyConnFromConfig(conn snet.Conn, resolver pathmgr.Resolver, localIA addr.IA) (net.PacketConn, error) {
	return NewPolicyConn(c, conn, resolver, localIA), nil
}
