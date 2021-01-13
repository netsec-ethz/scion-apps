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
	"testing"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	name      = "netcat"
	clientBin = "scion-netcat"
	serverBin = "scion-netcat"
)

func TestIntegrationScionNetcat(t *testing.T) {
	if err := integration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	// Start a scion-netcat server socket and query it with a scion-netcat client
	// Common arguments
	cmnArgs := []string{"-vv"}
	// Server
	serverPort := "1234"
	serverArgs := []string{"-l", serverPort}
	serverArgs = append(cmnArgs, serverArgs...)

	testMessage := "Hello World!"
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	clientBinWrapperCmd, err := wrapperCommand(tmpDir, fmt.Sprintf("echo -e '%s'", testMessage),
		integration.AppBinPath(clientBin))
	if err != nil {
		t.Fatalf("Failed to wrap scion-netcat input: %s\n", err)
	}
	clientCmd := clientBinWrapperCmd
	serverCmd := integration.AppBinPath(serverBin)

	// QUIC tests (default mode)
	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"client_help",
			append(cmnArgs, "--help"),
			nil,
			nil,
			integration.RegExp("^.*SCION.*$"),
			nil,
		},
		{
			"client_hello",
			append(cmnArgs, integration.DstAddrPattern+":"+serverPort),
			integration.RegExp(fmt.Sprintf("^%s$", testMessage)),
			nil,
			nil,
			integration.NoPanic(),
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
		IAPairs = IAPairs[:5]

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, 250*time.Millisecond); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}

func TestIntegrationScionNetcatUDP(t *testing.T) {
	// UDP tests
	// Common arguments
	cmnArgs := []string{"-vv", "-u"}

	// Server
	serverPort := "1234"
	serverArgs := []string{"-l", serverPort}
	serverArgs = append(cmnArgs, serverArgs...)

	testMessage := "Hello UDP World!"
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	clientBinWrapperCmd, err := wrapperCommand(tmpDir, fmt.Sprintf("echo -e '%s'", testMessage),
		integration.AppBinPath(clientBin))
	if err != nil {
		t.Fatalf("Failed to wrap scion-netcat input: %s\n", err)
	}
	clientCmd := clientBinWrapperCmd
	serverCmd := integration.AppBinPath(serverBin)

	testCases := []struct {
		Name              string
		Args              []string
		ServerOutMatchFun func(bool, string) bool
		ServerErrMatchFun func(bool, string) bool
		ClientOutMatchFun func(bool, string) bool
		ClientErrMatchFun func(bool, string) bool
	}{
		{
			"client_hello_UDP",
			append(cmnArgs, integration.DstAddrPattern+":"+serverPort),
			nil,
			nil,
			integration.RegExp("^.*Connected.*$"),
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
		IAPairs = IAPairs[:5]

		if err := integration.RunTests(in, IAPairs, integration.DefaultClientTimeout, 250*time.Millisecond); err != nil {
			t.Fatalf("Error during tests err: %v", err)
		}
	}
}

func wrapperCommand(tmpDir string, inputSource string, command string) (wrapperCmd string, err error) {
	wrapperCmd = path.Join(tmpDir, fmt.Sprintf("%s_wrapper.sh", serverBin))
	f, err := os.OpenFile(wrapperCmd, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to create %s: %v", wrapperCmd, err))
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	_, _ = w.WriteString(fmt.Sprintf("#!/bin/bash\ntimeout 5 /bin/bash -c \"%s | %s $1 $2 $3 $4\" || true",
		inputSource, command))
	return wrapperCmd, nil
}
