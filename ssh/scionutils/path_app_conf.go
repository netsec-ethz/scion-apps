package scionutils

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/snet"
	"net"
)

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

func (c *PathAppConf) ConnWrapperFromConfig(conn snet.Conn) (net.PacketConn, error) {
	connWrapper := NewPolicyConn(conn, c)
	switch c.pathSelection {
	case Static:
		return NewStaticPolicyConn(*connWrapper), nil
	case RoundRobin:
		return NewRoundRobinPolicyConn(*connWrapper), nil
	case Arbitrary:
		return connWrapper, nil
	default:
		return nil, common.NewBasicError("PathAppConf: Unable to create ConnWrapper for given configuration", nil)
	}
}

// PathSelection is an enum-like struct which serves as a convenient representation for user-specified path selection mode
// Arbitrary: arbitrary path selection
// Static: use the first selected path for the whole connection
// RoundRobin: iterate through available paths in a circular fashion
// Random: randomized path selection, currently not supported
type PathSelection int

const (
	Arbitrary  PathSelection = 0
	Static     PathSelection = 1
	RoundRobin PathSelection = 2
	Random     PathSelection = 3
)

func PathSelectionFromString(s string) (PathSelection, error) {
	selectionMap := map[string]PathSelection{
		"arbitrary":   Arbitrary,
		"static":      Static,
		"round-robin": RoundRobin,
		"random":      Random,
	}
	pathSelection, ok := selectionMap[s]
	if !ok {
		return 0, common.NewBasicError("Unknown path selection option", nil)
	}
	return pathSelection, nil
}
