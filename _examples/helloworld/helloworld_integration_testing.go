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
	"time"

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
	cmnArgs := []string{}
	clientArgs := []string{
		"-remote", integration.DstAddrPattern + ":" + "12345"}
	clientArgs = append(clientArgs, cmnArgs...)
	serverArgs := []string{"-port", "12345"}
	serverArgs = append(serverArgs, cmnArgs...)

	in := integration.NewAppsIntegration(name, cmd, clientArgs, serverArgs, "")
	IAPairs := integration.IAPairs(integration.DispAddr)
	clientTimeout := 10*time.Second
	if err := integration.RunTests(in, IAPairs, clientTimeout); err != nil {
		t.Fatalf("Error during tests err: %v", err)
	}
}

