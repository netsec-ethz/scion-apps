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

// +build scion_integration

package main

import (
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name = "helloworld_integration"
	cmd  = "helloworld"
)

func TestHelloworldSample(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	// Common arguments
	cmnArgs := []string{}
	// Client
	clientArgs := []string{
		"-remote", integration.DstAddrPattern + ":" + "12345"}
	clientArgs = append(clientArgs, cmnArgs...)
	// Server
	serverArgs := []string{"-port", "12345"}
	serverArgs = append(serverArgs, cmnArgs...)

	in := integration.NewAppsIntegration(name, cmd, clientArgs, serverArgs, "")
	// Host address pattern
	hostAddr := integration.HostAddr
	// Cartesian product of src and dst IAs, is a random permutation
	// can be restricted to a subset to reduce the number of tests to run without significant
	// loss of coverage
	IAPairs := integration.IAPairs(hostAddr)[:5]
	// Run the tests to completion or until a test fails,
	// increase the client timeout if clients need more time to start
	if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout); err != nil {
		t.Fatalf("Error during tests err: %v", err)
	}
}

