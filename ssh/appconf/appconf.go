package appconf

import (
"github.com/scionproto/scion/go/lib/assert"
"github.com/scionproto/scion/go/lib/common"
"github.com/scionproto/scion/go/lib/overlay"
"github.com/scionproto/scion/go/lib/pathpol"
"github.com/scionproto/scion/go/lib/spath"
)

//Defines configuration options for user-defined path policies

type AppConf struct {
	policy *pathpol.Policy
	pathSelection PathSelection
	staticPath *spath.Path
	staticNextHop *overlay.OverlayAddr
	Test int
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
	assert.Must(c.pathSelection.IsStatic(), "Must not access static path while path selection is not static")
	return c.staticNextHop, c.staticPath
}

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


func (ps PathSelection) IsStatic() bool {
	return ps == Static
}

func (ps PathSelection) IsArbitrary() bool {
	return ps == Arbitrary
}

func (ps PathSelection) IsRoundRobin() bool {
	return ps == RoundRobin
}

func (ps PathSelection) IsRandom() bool {
	return ps == Random
}



