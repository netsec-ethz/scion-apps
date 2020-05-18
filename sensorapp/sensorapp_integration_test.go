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
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name      = "sensorserver"
	clientBin = "scion-sensorfetcher"
	serverBin = "scion-sensorserver"
)

func TestIntegrationSensorserver(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	// Common arguments
	cmnArgs := []string{}
	// Server
	serverPort := "42003"
	serverArgs := []string{}
	serverArgs = append(serverArgs, cmnArgs...)

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	serverBinWrapperCmd, err := wrapperCommand(tmpDir, "timereader.py",
		integration.AppBinPath(serverBin), serverPort)
	if err != nil {
		t.Fatalf("Failed to wrap sensorserver input: %s\n", err)
	}
	serverCmd := serverBinWrapperCmd

	// Client
	clientCmd := integration.AppBinPath(clientBin)

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
			integration.RegExp("^20\\d{2}\\/\\d{2}\\/\\d{2} \\d{2}:\\d{2}:\\d{2}$"),
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

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, integration.DefaultClientTimeout/5); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}

func wrapperCommand(tmpDir string, inputSource string, command string, port string) (wrapperCmd string, err error){
	wrapperCmd =  path.Join(tmpDir, fmt.Sprintf("%s_wrapper.sh", serverBin))
	f, err := os.OpenFile(wrapperCmd, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to create %s: %v", wrapperCmd, err))
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	_, file, _, _ := runtime.Caller(0)
	cwd := path.Dir(file)
	inputSource = path.Join(cwd, "sensorserver", inputSource)
	_, _ = w.WriteString(fmt.Sprintf("#!/bin/bash\ntimeout 5 /bin/bash -c \"%s | %s -p %s\"",
		inputSource, command, port))
	return wrapperCmd, nil
}
