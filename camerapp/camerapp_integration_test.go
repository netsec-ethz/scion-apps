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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name      = "camerapp"
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
	serverArgs := []string{"-p", serverPort, "-d", integration.AppTestdataPath()}
	serverArgs = append(serverArgs, cmnArgs...)

	// Sample file path
	sample := path.Join(integration.AppTestdataPath(), "logo.jpg")
	// Image fetcher output directory
	sampleOutputDir, err := ioutil.TempDir("", fmt.Sprintf("%s_integration_output", name))
	sampleOuput := path.Join(sampleOutputDir, "download.jpg")
	if err != nil {
		t.Fatalf("Error during setup err: %v", err)
	}

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
			append([]string{"-s", integration.DstAddrPattern + ":" + serverPort, "-output", sampleOuput}, cmnArgs...),
			nil,
			nil,
			func(prev bool, line string) bool {
				if !prev {
					matched, err := regexp.MatchString("^r+[.r]+$", line)
					if err == nil {
						return matched
					}
				} else {
					matched, err := regexp.MatchString("^Done, exiting. Total duration \\d+\\.\\d+m?s$", line)
					if err != nil || !matched {
						return false
					}
					// The image was downloaded, compare it with the source
					cmd := exec.Command("cmp", "-l", sampleOuput, sample)
					// cmp exits with 0 exit status if the files are identical, and err is nil if the exit status is 0
					err = cmd.Run()
					return err == nil
				}
				return prev
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

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, 0); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
	// Cleanup temporary output directory
	if err := os.RemoveAll(sampleOutputDir); err != nil {
		fmt.Printf("Error during cleanup: err=%s\n", err)
	}
}
