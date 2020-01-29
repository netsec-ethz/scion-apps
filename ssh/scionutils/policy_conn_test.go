package scionutils

import (
	"testing"
)

//All tests in this file test the correctness of the path selection modes (round-robin, static)
//The assumption is that path filtering has already been tested in SCIONProto

//const numPaths = 5 // number of paths returned by mock resolver

func TestStaticPolicyConn_SelectPath(t *testing.T) {
	/*
		pc := NewPolicyConn(&PathAppConf{pathSelection: Static}, nil)

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
	*/
}

func TestRoundRobinPolicyConn_SelectPath(t *testing.T) {
	/*
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
	*/
}
