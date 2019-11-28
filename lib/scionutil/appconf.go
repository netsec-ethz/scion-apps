package scionutil

import (
"github.com/scionproto/scion/go/lib/assert"
"github.com/scionproto/scion/go/lib/common"
"github.com/scionproto/scion/go/lib/overlay"
"github.com/scionproto/scion/go/lib/pathpol"
"github.com/scionproto/scion/go/lib/spath"
)

// AppConf represents application configurations specified by the user using command-line arguments
// policy: SCION path policy
// pathSelection: path selection mode
type AppConf struct {
	policy *pathpol.Policy
	pathSelection PathSelection
	staticPath *spath.Path
	staticNextHop *overlay.OverlayAddr
}

func NewAppConf(policy *pathpol.Policy, pathSelection string) (*AppConf, error) {
	ps, err := PathSelectionFromString(pathSelection)
	if err != nil {
		return nil, err
	}
	return &AppConf{
		policy:        policy,
		pathSelection: ps,
	}, nil
}

func (c *AppConf) PathSelection() PathSelection {
	return c.pathSelection
}

func (c *AppConf) Policy () *pathpol.Policy {
	return c.policy
}

func (c *AppConf) SetStaticPath (nh *overlay.OverlayAddr, sp *spath.Path)  {
	c.staticNextHop, c.staticPath = nh, sp
}

func (c *AppConf) GetStaticPath () (*overlay.OverlayAddr, *spath.Path) {
	assert.Must(c.pathSelection == Static, "AppConf: Must not access static path while path selection is not static")
	return c.staticNextHop, c.staticPath
}

// PathSelection is an enum-like struct which serves as a convenient representation for user-specified path selection mode
// Arbitrary: arbitrary path selection
// Static: use the first selected path for the whole connection
// RoundRobin: iterate through available paths in a circular fashion
// Random: randomized path selection, currently not supported
type PathSelection int

const (
	Arbitrary PathSelection = 0
	Static PathSelection = 1
	RoundRobin PathSelection = 2
	Random PathSelection = 3
)

func PathSelectionFromString (s string) (PathSelection, error) {
	selectionMap := map[string]PathSelection {
		"arbitrary" : Arbitrary,
		"static": Static,
		"round-robin": RoundRobin,
		"random": Random,
	}
	pathSelection, ok := selectionMap[s]
	if !ok {
		return 0, common.NewBasicError("Unknown path selection option", nil )
	}
	return pathSelection, nil
}




