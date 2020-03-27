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
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
)

func TestIntegrationBwtestclient(t *testing.T) {
	// Start a bwtestserver and query it with the bwtestclient
	var commands []*exec.Cmd
	defer func() {
		// cleanup after test
		for _, cmd := range commands {
			if err := cmd.Process.Kill(); err != nil {
				fmt.Printf("Failed to kill process: %v", err)
			}
		}
	}()

	// Server command
	cmd := exec.Command("bwtestserver")
	serverOut, _ := cmd.StdoutPipe()
	serverStdoutScanner := bufio.NewScanner(serverOut)
	log.Info("Start server", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during server startup %v: %v", "bwtestserver", err)
	}
	commands = append(commands, cmd)
	time.Sleep(250 * time.Millisecond)

	// Client command
	cmd = exec.Command("bwtestclient",
		"-s",
		"1-ff00:0:110,[127.0.0.1]:40002",
		"-cs",
		"1Mbps",
	)
	clientStdout, _ := cmd.StdoutPipe()
	log.Info("Run client", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during client query %v: %v", "bwtestclient", err)
	}
	// commands = append(commands, cmd)
	// client exits after a single query, no need to terminate it
	clientOut, _ := ioutil.ReadAll(clientStdout)
	cmd.Wait()

	// Check client output
	// We expect a 0% loss rate and reaching the full bandwidth
	if strings.Contains(string(clientOut), "Loss rate: 0 %") &&
	   strings.Contains(string(clientOut), "Achieved bandwidth: 1000000 bps / 1.00 Mbps") {
		fmt.Println("Client succeeded.")
	} else {
		t.Fatalf("Client failed. Output=%s", clientOut)
	}

	// Check server output
	fullServerOutput := ""
	for serverStdoutScanner.Scan() {
		serverOutput := serverStdoutScanner.Text()
		fullServerOutput += fmt.Sprintln(serverOutput)
		if strings.Contains(serverOutput, "Received request") {
			fmt.Println("Server received client request.")
			break
		}
	}
	if err := serverStdoutScanner.Err(); err != nil {
		t.Fatalf("Server failed to start bandwidth test. Output=%s", fullServerOutput)
	}
}

