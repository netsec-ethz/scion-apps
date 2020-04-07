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

func TestSCIONNetcat(t *testing.T) {
	// Start a scion-netcat server socket and query it with a scion-netcat client
	var commands []*exec.Cmd
	defer func() {
		// cleanup after test
		for _, cmd := range commands {
			if err := cmd.Process.Kill(); err != nil {
				fmt.Printf("Failed to kill process: %v", err)
			}
		}
	}()

	// Check we are running the right scion-netcat, inspect the help
	cmd := exec.Command("netcat",
		"--help",
	)
	ncOut, _ := cmd.StdoutPipe()
	log.Info("Run netcat help", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during help %v: %v", "netcat", err)
	}
	ncStdOutput, _ := ioutil.ReadAll(ncOut)
	cmd.Wait()

	// Check help output
	if strings.Contains(string(ncStdOutput), "SCION") {
		fmt.Println("Using scion-netcat")
	} else {
		t.Fatalf("Wrong netcat on PATH. Output=%s", ncStdOutput)
	}

	// Start the actual test
	testMessage := "Hello World!"
	// Server command
	cmd = exec.Command("netcat",
		"-l",
		"1234",
	)
	serverOut, _ := cmd.StdoutPipe()
	serverStdoutScanner := bufio.NewScanner(serverOut)
	log.Info("Start server", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during netcat server socket listen %v: %v", "netcat", err)
	}
	commands = append(commands, cmd)
	time.Sleep(250 * time.Millisecond)

	// Echo command
	echoCmd := exec.Command("echo",
		"-e",
		fmt.Sprintf("\n\n%s\n", testMessage),
	)
	echoOut, _ := echoCmd.StdoutPipe()

	// Client command
	cmd = exec.Command("netcat",
		"1-ff00:0:110,[127.0.0.1]:1234",
	)
	cmd.Stdin = echoOut
	log.Info("Run client", "cmd", fmt.Sprintf("%s %s", cmd.Path, cmd.Args))
	if err := cmd.Start(); err != nil {
		t.Fatalf("Error during netcat piping on client side %v: %v", "netcat", err)
	}
	commands = append(commands, cmd)

	// Run echo piped into netcat client
	if err := echoCmd.Run(); err != nil {
		t.Fatalf("Error during echo to netcat pipe on client side %v: %v", "echo | netcat", err)
	}

	// Check server output
	fullServerOutput := ""
	for serverStdoutScanner.Scan() {
		serverOutput := serverStdoutScanner.Text()
		fullServerOutput += fmt.Sprintln(serverOutput)
		if strings.Contains(serverOutput, testMessage) {
			fmt.Printf("Server received client message: %s\n", serverOutput)
			break
		}
	}
	if err := serverStdoutScanner.Err(); err != nil {
		t.Fatalf("Server failed to receive `%s`. Output=%s", testMessage, fullServerOutput)
	}
}

