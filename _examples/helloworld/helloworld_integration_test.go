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

//go:build integration
// +build integration

package main

import (
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	bin = "example-helloworld"
)

func TestMain(m *testing.M) {
	integration.TestMain(m)
}

func TestHelloworldSample(t *testing.T) {
	cmd := integration.AppBinPath(bin)
	// Server
	serverPortOffset := 12345
	serverArgs := []string{"-listen", ":" + integration.ServerPortReplace}

	// Client
	clientArgs := []string{
		"-remote", integration.DstAddrPattern + ":" + integration.ServerPortReplace,
	}

	in := integration.NewAppsIntegration(cmd, cmd, clientArgs, serverArgs)
	in.ServerOutMatch = integration.RegExp("(?m)^Received .*: hello world .*\nWrote 24 bytes")
	in.ClientOutMatch = integration.RegExp("(?m)^Wrote 22 bytes.\nReceived reply: take it back! .*")
	// Cartesian product of src and dst IAs, a random permutation
	// restricted to a subset to reduce the number of tests to run without significant
	// loss of coverage
	iaPairs := integration.DefaultIAPairs()
	// Add different ports to servers.
	integration.AssignUniquePorts(iaPairs, serverPortOffset, 2)
	// Run the tests to completion or until a test fails,
	// increase the ClientTimeout if clients need more time to start
	if err := in.Run(t, iaPairs); err != nil {
		t.Error(err)
	}
}
