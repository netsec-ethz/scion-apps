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
	"testing"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name = "boingboing"
	bin  = "example-boingboing"
)

func TestIntegrationBoingboing(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	cmd := integration.AppBinPath(bin)
	// Server
	serverPort := "12345"
	serverArgs := []string{"-mode", "server", "-port", serverPort}

	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"default",
			[]string{"-remote", integration.DstAddrPattern + ":" + serverPort,
				"-count", "10", "-interval", "0.01s"},
			integration.RegExp("Received message"),
			nil,
			integration.RegExp("Received reply.*seq=9"),
			nil,
		},
	}

	for _, tc := range testCases {
		in := integration.NewAppsIntegration(name, tc.Name, cmd, cmd, tc.Args, serverArgs, true)
		in.ServerStdout(tc.ServerOutMatchFun)
		in.ServerStderr(tc.ServerErrMatchFun)
		in.ClientStdout(tc.ClientOutMatchFun)
		in.ClientStderr(tc.ClientErrMatchFun)

		IAPairs := integration.IAPairs(integration.HostAddr)

		if err := integration.RunTests(in, IAPairs, 30*time.Second, 0); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}
