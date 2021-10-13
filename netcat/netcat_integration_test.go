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
	"fmt"
	"testing"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	netcatBin = "scion-netcat"
)

func TestMain(m *testing.M) {
	integration.TestMain(m)
}

// TestIntegrationScionNetcatCmd runs the netcat listeners in -c mode, returning a
// fixed string for each newly connected client.
// This mode is easiest to test here as it does not require any stdin/out redirections
// and the clients can terminate succesfully without interrupting them.
// XXX: This is testing the "happy" path only, meaning pretty much anything
// else does not currently work.
func TestIntegrationScionNetcatCmd(t *testing.T) {
	netcatCmd := integration.AppBinPath(netcatBin)

	cases := []struct {
		name    string
		message string
		flags   []string
	}{
		{
			name:    "QUIC",
			message: "Hello QUIC World!",
			flags:   nil,
		},
		{
			name:    "UDP",
			message: "Hello UDP World!",
			flags:   []string{"-u"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverPort := "1234"
			serverArgs := concat(
				tc.flags,
				[]string{"-b", "-K", "-c", "echo " + tc.message, "-l", serverPort},
			)
			// NOTE: we need -b as the client does not otherwise send any data to make the "query"
			// BUG: should also work with -k, but doesn't (!?)

			clientScriptArgs := concat(
				tc.flags,
				[]string{"-b", "-q", "10ms", integration.DstAddrPattern + ":" + serverPort},
			)
			in := integration.NewAppsIntegration(netcatCmd, netcatCmd, clientScriptArgs, serverArgs)
			in.ClientDelay = 250 * time.Millisecond
			in.ClientOutMatch = integration.RegExp(fmt.Sprintf("(?m)^%s$", tc.message))

			iaPairs := integration.DefaultIAPairs()
			if err := in.Run(t, iaPairs); err != nil {
				t.Error(err)
			}
		})
	}
}

func concat(slices ...[]string) []string {
	var r []string
	for _, s := range slices {
		r = append(r, s...)
	}
	return r
}
