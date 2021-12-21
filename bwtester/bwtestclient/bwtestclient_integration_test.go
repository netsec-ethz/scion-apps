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
	clientBin = "scion-bwtestclient"
	serverBin = "scion-bwtestserver"
)

func TestMain(m *testing.M) {
	integration.TestMain(m)
}

func TestIntegrationBwtestclient(t *testing.T) {
	clientCmd := integration.AppBinPath(clientBin)
	serverCmd := integration.AppBinPath(serverBin)

	// Server
	serverArgs := []string{}
	// Client
	clientArgs := []string{"-s", integration.DstAddrPattern + ":40002", "-cs", "1,?,?,1Mbps"}

	in := integration.NewAppsIntegration(clientCmd, serverCmd, clientArgs, serverArgs)
	in.ServerOutMatch = integration.Contains("Received request")
	in.ClientOutMatch = integration.RegExp("(?m)^Achieved bandwidth: \\d+ bps / \\d+.\\d+ [Mk]bps$")

	iaPairs := integration.DefaultIAPairs()
	if err := in.Run(t, iaPairs); err != nil {
		t.Error(err)
	}
}
