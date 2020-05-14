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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name      = "sensorserver"
	clientBin = "scion-sensorfetcher"
	serverBin = "scion-sensorserver"
)

func wrapperCommand(inputSource string, command string, port int) (wrapperCmd string, err error){
	wrapperCmd = integration.AppBinPath(fmt.Sprintf("%s_wrapper.sh", serverBin))
	f, err := os.OpenFile(serverBinWrapper, O_RDWR|O_CREATE|O_TRUNC, 0777)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create %s", serverBinWrapperCmd))
	}
	w := bufio.NewWriter(f)
	defer w.Flush()
	_, _ = w.WriteString(fmt.Sprintf("%s | %s -p %d", inputSource, command, port),
		inputSource, command, port)
	return wrapperCmd, nil
}

func TestIntegrationSensorserver(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	// Common arguments
	cmnArgs := []string{}
	// Server
	serverPort := "42003"
	serverArgs := []string{"-p", serverPort}
	serverArgs = append(serverArgs, cmnArgs...)

	clientCmd := integration.AppBinPath(clientBin)
	serverBinWrapperCmd, err := wrapperCommand("./sensorapp/sensorserver/timereader.py",
		integration.AppBinPath(serverBin), serverPort)
	if err != nil {
		t.Fatalf("Failed to wrap sensorserver input: %s\n", err)
	}
	serverCmd := integration.AppBinPath(serverBinWrapperCmd)

	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"fetch_time",
			append([]string{"-s", integration.DstAddrPattern + ":" + serverPort}, cmnArgs...),
			nil,
			nil,
			integration.RegExp("^\\w{3} \\w+ \\d+ \\d{2}:\\d{2}:\\d{2} UTC 20\\d{2}$"),
			nil,
		},
	}

	for _, tc := range testCases {
		in := integration.NewAppsIntegration(name, tc.Name, clientCmd, serverCmd, tc.Args, serverArgs, true)
		in.ServerStdout(tc.ServerOutMatchFun)
		in.ServerStderr(tc.ServerErrMatchFun)
		in.ClientStdout(tc.ClientOutMatchFun)
		in.ClientStderr(tc.ClientErrMatchFun)

		hostAddr := integration.HostAddr

		IAPairs := integration.IAPairs(hostAddr)
		IAPairs = IAPairs[:1]

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}
