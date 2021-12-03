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
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	clientBin = "scion-sensorfetcher"
	serverBin = "scion-sensorserver"
)

func TestMain(m *testing.M) {
	integration.TestMain(m)
}

func TestIntegrationSensorserver(t *testing.T) {
	// Server
	serverPort := "42003"
	serverArgs := []string{"-p", serverPort}

	scriptServerWithInput := integration.InputPipeScript(
		t.TempDir(),
		"sensorserver",
		"sensorserver/timereader.py",
		integration.AppBinPath(serverBin),
	)

	// Client
	clientCmd := integration.AppBinPath(clientBin)
	clientArgs := []string{"-s", integration.DstAddrPattern + ":" + serverPort}

	in := integration.NewAppsIntegration(clientCmd, scriptServerWithInput, clientArgs, serverArgs)
	in.ClientOutMatch = integration.RegExp(`^20\d{2}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}` + "\n")
	in.ClientDelay = 250 * time.Millisecond

	iaPairs := integration.DefaultIAPairs()
	if err := in.Run(t, iaPairs); err != nil {
		t.Error(err)
	}
}
