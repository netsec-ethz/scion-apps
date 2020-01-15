package scionutils

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/pathpol"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

//All tests in this file test the correctness of the path selection modes (round-robin, static)
//The assumption is that path filtering has already been tested in SCIONProto

const numPaths = 5 // number of paths returned by mock resolver
var selectedPathMap map[*spathmeta.AppPath]selectedPath

type selectedPath struct {
	*overlay.OverlayAddr
	*spath.Path
}

type MockPathResolver struct {
}

func (pr MockPathResolver) Query(ctx context.Context, src, dst addr.IA, flags sciond.PathReqFlags) spathmeta.AppPathSet {
	return nil
}

func (pr MockPathResolver) QueryFilter(ctx context.Context, src, dst addr.IA, policy *pathpol.Policy) spathmeta.AppPathSet {

	set := spathmeta.AppPathSet{}
	selectedPathMap = map[*spathmeta.AppPath]selectedPath{}

	for i := 0; i < numPaths; i++ {
		appPath := &spathmeta.AppPath{
			Entry: &sciond.PathReplyEntry{
				Path: &sciond.FwdPathMeta{
					FwdPath:    nil,
					Mtu:        0,
					Interfaces: nil,
					ExpTime:    0,
				},
			},
		}
		set[spathmeta.PathKey(strconv.Itoa(i))] = appPath
		selectedPathMap[appPath] = selectedPath{
			OverlayAddr: &overlay.OverlayAddr{},
			Path:        &spath.Path{},
		}
	}
	fmt.Println(set[spathmeta.PathKey("1")] == set[spathmeta.PathKey("2")])
	return set
}

func (pr MockPathResolver) Watch(ctx context.Context, src, dst addr.IA) (*pathmgr.SyncPaths, error) {
	return nil, nil
}

func (pr MockPathResolver) WatchFilter(ctx context.Context, src, dst addr.IA, filter *pathpol.Policy) (*pathmgr.SyncPaths, error) {
	return nil, nil
}

func (pr MockPathResolver) WatchCount() int {
	return 0
}
func (MockPathResolver) RevokeRaw(ctx context.Context, rawSRevInfo common.RawBytes) {}

func (MockPathResolver) Revoke(ctx context.Context, sRevInfo *path_mgmt.SignedRevInfo) {}

func (MockPathResolver) Sciond() sciond.Connector { return nil }

func mockGetPath(appPath *spathmeta.AppPath) (*overlay.OverlayAddr, *spath.Path, error) {
	entry := selectedPathMap[appPath]
	return entry.OverlayAddr, entry.Path, nil
}
func TestRoundRobinPolicyConn_SelectPath(t *testing.T) {

	pc := roundRobinPolicyConn{}
	pc.pathResolver = MockPathResolver{}
	pc.conf = &PathAppConf{}
	pc.pathConverter = mockGetPath
	var paths []selectedPath

	ia, err := addr.IAFromString("1-ff00:0:1")
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < numPaths*10; i++ {
		nextHop, path, err := pc.SelectPath(snet.Addr{IA: ia})
		if err != nil {
			// we don't care about path parsing errors
			if !strings.Contains(err.Error(), ErrBadOverlay) && !strings.Contains(err.Error(), ErrInitPath) {
				t.Errorf("Error selecting path: %s", err)
			}
		}
		paths = append(paths, selectedPath{nextHop, path})
	}

	for i := 0; i < len(paths)-numPaths; i++ {
		if paths[i].Path != paths[i+numPaths].Path {
			t.Errorf("Paths indeces %d and %d, should be equal ", i, i+numPaths)
		}
	}
}
