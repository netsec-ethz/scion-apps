package scionutils

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/hostinfo"
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
					FwdPath:    common.RawBytes{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
					Mtu:        0,
					Interfaces: nil,
					ExpTime:    0,
				},
				HostInfo: hostinfo.HostInfo{
					Port: 1,
					Addrs: struct {
						Ipv4 []byte
						Ipv6 []byte
					}{
						[]byte{1, 1, 1, 1},
						[]byte{},
					},
				},
			},
		}
		set[spathmeta.PathKey(strconv.Itoa(i))] = appPath
		selectedPathMap[appPath] = selectedPath{
			OverlayAddr: &overlay.OverlayAddr{},
			Path:        &spath.Path{},
		}
	}
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

func TestStaticPolicyConn_SelectPath(t *testing.T) {
	pc := NewPolicyConn(&PathAppConf{pathSelection: Static}, nil, MockPathResolver{}, addr.IA{})
	ia, err := addr.IAFromString("1-ff00:0:1")
	if err != nil {
		t.Error(err)
	}

	appPath := pc.(*policyConn).pathSelector.SelectPath(snet.Addr{IA: ia})
	if err != nil {
		t.Fatalf("Error selecting path: %s", err)
	}

	for i := 0; i < numPaths*10; i++ {
		newAppPath := pc.(*policyConn).pathSelector.SelectPath(snet.Addr{IA: ia})
		if err != nil {
			t.Fatalf("Error selecting path: %s", err)
		}

		if newAppPath != appPath {
			t.Fatalf("Static path selection: Expected static path %v, found path %v", appPath, newAppPath)
		}
	}

}

func TestRoundRobinPolicyConn_SelectPath(t *testing.T) {
	pc := NewPolicyConn(&PathAppConf{pathSelection: RoundRobin}, nil, MockPathResolver{}, addr.IA{})
	var paths []*spathmeta.AppPath

	ia, err := addr.IAFromString("1-ff00:0:1")
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < numPaths*10; i++ {
		appPath := pc.(*policyConn).pathSelector.SelectPath(snet.Addr{IA: ia})
		if err != nil {
			t.Fatalf("Error selecting path: %s", err)
		}
		paths = append(paths, appPath)
	}
	// the first numPaths paths must be different:
	pathSet := map[string]struct{}{}
	for _, p := range paths[:numPaths] {
		pathSet[string(p.Entry.Path.FwdPath)] = struct{}{}
	}
	if len(pathSet) != numPaths {
		t.Fatalf("Expected %d different paths; got only %d", numPaths, len(pathSet))
	}
	// check the sequence of paths in "paths" repeats (round robin):
	for i := 0; i < len(paths)-numPaths; i++ {
		if bytes.Compare(paths[i].Entry.Path.FwdPath, paths[i+numPaths].Entry.Path.FwdPath) != 0 {
			t.Fatalf("Paths at indices %d and %d, should be equal ", i, i+numPaths)
		}
	}
}
