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
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name      = "cameraapp"
	clientBin = "scion-imagefetcher"
	serverBin = "scion-imageserver"
)

func TestIntegrationImagefetcher(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	clientCmd := integration.AppBinPath(clientBin)
	serverCmd := integration.AppBinPath(serverBin)

	// Common arguments
	cmnArgs := []string{}
	// Server
	serverPort := "42002"
	serverArgs := []string{"-p", serverPort}
	serverArgs = append(serverArgs, cmnArgs...)

	_, file, _, _ := runtime.Caller(0)
	cwd := path.Dir(file)
	// Sample file path
	sample := path.Join(cwd, "../imageserver/logo.jpg")

	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"fetch_image",
			append([]string{"-s", integration.DstAddrPattern + ":" + serverPort, "-output", "./download.jpg"}, cmnArgs...),
			nil,
			nil,
			integration.RegExp("^Done, exiting. Total duration \\d+\\.\\d+m?s$"),
			nil,
		},
		{
			"fetch_image",
			append([]string{"-s", integration.DstAddrPattern + ":" + serverPort, "-output", "/tmp/download.jpg"}, cmnArgs...),
			nil,
			nil,
			func(prev bool, line string) (res bool) {
				if !prev {
					matched, err := regexp.MatchString("^r+[.r]+$", line)
					if err == nil {
						res = matched
					}
				} else {
					matched, err := regexp.MatchString("^Done, exiting. Total duration \\d+\\.\\d+m?s$", line)
					if err != nil || !matched {
						return false
					}
					res = true
					// The image was downloaded, compare it with the source
					cmd := exec.Command("cmp", "-l", "/tmp/download.jpg", sample)
					// cmp exits with 0 exit status if the files are identical, and err is nil if the exit status is 0
					err = cmd.Run()
					res = err == nil
				}
				return res
			},
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

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, 0); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}
