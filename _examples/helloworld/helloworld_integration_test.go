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

func TestHelloworldSample(t *testing.T) {
	// Start a helloworld server and query it with the client
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
	cmd := exec.Command("helloworld",
		"-port",
		"12345",
	)
	serverOut, _ := cmd.StdoutPipe()
	serverStdoutScanner := bufio.NewScanner(serverOut)
	log.Info("Start server", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during server startup %v: %v", "helloworld", err)
	}
	commands = append(commands, cmd)
	time.Sleep(250 * time.Millisecond)

	// Client command
	cmd = exec.Command("helloworld",
		"-remote",
		"1-ff00:0:110,[127.0.0.1]:12345",
	)
	clientOut, _ := cmd.StdoutPipe()
	log.Info("Run client", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during client query %v: %v", "helloworld", err)
	}
	// commands = append(commands, cmd)
	// client exits after a single query, no need to terminate it
	clientStdOutput, _ := ioutil.ReadAll(clientOut)
	cmd.Wait()

	// Check client output
	if strings.Contains(string(clientStdOutput), "Done.") {
		fmt.Println("Client succeeded.")
	} else {
		t.Fatalf("Client failed. Output=%s", clientStdOutput)
	}

	// Check server output
	fullServerOutput := ""
	for serverStdoutScanner.Scan() {
		serverOutput := serverStdoutScanner.Text()
		fullServerOutput += fmt.Sprintln(serverOutput)
		if strings.Contains(serverOutput, "hello world") {
			fmt.Println("Server received client query.")
			break
		}
	}
	if err := serverStdoutScanner.Err(); err != nil {
		t.Fatalf("Server failed to receive `hello world`. Output=%s", fullServerOutput)
	}
}

