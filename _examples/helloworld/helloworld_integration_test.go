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

// +build integration

package main

import (
	"strings"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name = "helloworld"
	bin  = "example-helloworld"
)

func TestHelloworldSample(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	cmd := integration.AppBinPath(bin)
	// Common arguments
	cmnArgs := []string{}
	// Server
	serverPort := "12345"
	serverArgs := []string{"-port", serverPort}
	serverArgs = append(serverArgs, cmnArgs...)

	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"client_hello",
			append([]string{"-remote", integration.DstAddrPattern + ":" + serverPort}, cmnArgs...),
			func(prev bool, line string) bool {
				res := strings.Contains(line, "hello world")
				return prev || res // return true if any output line contains the string
			},
			nil,
			integration.Contains("Done. Wrote 11 bytes."),
			nil,
		},
	}

	for _, tc := range testCases {
		in := integration.NewAppsIntegration(name, tc.Name, cmd, cmd, tc.Args, serverArgs, true)
		in.ServerStdout(tc.ServerOutMatchFun)
		in.ServerStderr(tc.ServerErrMatchFun)
		in.ClientStdout(tc.ClientOutMatchFun)
		in.ClientStderr(tc.ClientErrMatchFun)
		// Host address pattern
		hostAddr := integration.HostAddr
		// Cartesian product of src and dst IAs, is a random permutation
		// can be restricted to a subset to reduce the number of tests to run without significant
		// loss of coverage
		IAPairs := integration.IAPairs(hostAddr)
		IAPairs = IAPairs[:len(IAPairs)/2]
		// Run the tests to completion or until a test fails,
		// increase the client timeout if clients need more time to start
		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, 0); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}
