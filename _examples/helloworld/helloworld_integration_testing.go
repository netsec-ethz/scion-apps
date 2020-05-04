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

// +build scion_integration

package main

import (
	"fmt"
	"testing"
	"time"

	sintegration "github.com/scionproto/scion/go/lib/integration"
	"github.com/netsec-ethz/scion-apps/pkg/integration"
	"github.com/scionproto/scion/go/lib/log"
)

const (
	name = "helloworld_integration"
	cmd  = "helloworld"
	//cmd  = "/bin/bash"
	//cmd  = "/bin/echo"
	//cmd  = "/usr/bin/env"
)

func TestHelloworldSample(t *testing.T) {
	if err := sintegration.Init(name); err != nil {
		t.Fatalf("Failed to init: %s\n", err)
	}
	defer log.HandlePanic()
	defer log.Flush()
	cmnArgs := []string{}
	clientArgs := []string{
	//	}
	//	"Test message"}
	//	"-c", "/bin/echo", "$SCION_DAEMON_ADDRESS"}
	//	"--help"}
	//	"-remote", sintegration.DstAddrPattern + ":" + sintegration.ServerPortReplace}
		"-remote", sintegration.DstAddrPattern + ":" + "12345"}
	clientArgs = append(clientArgs, cmnArgs...)
	//serverArgs := []string{"--help"}
	serverArgs := []string{"-port", "12345"}
	//serverArgs := []string{"-c", "/bin/echo", "daemo $SCION_DAEMON_ADDRESS daemon"}
	//serverArgs := []string{"$SCION_DAEMON_ADDRESS"}
	//serverArgs := []string{}
	serverArgs = append(serverArgs, cmnArgs...)
	in := integration.NewAppsIntegration(name, cmd, clientArgs, serverArgs)
	if err := runTests(in, sintegration.IAPairs(sintegration.DispAddr)); err != nil {
		t.Fatalf("Error during tests err: %v", err)
	}
}

// RunTests runs the client and server for each IAPair.
// In case of an error the function is terminated immediately.
func runTests(in sintegration.Integration, pairs []sintegration.IAPair) error {
	return sintegration.ExecuteTimed(in.Name(), func() error {
		// First run all servers
		dsts := sintegration.ExtractUniqueDsts(pairs)
		for _, dst := range dsts {
			c, err := sintegration.StartServer(in, dst)
			if err != nil {
				log.Error(fmt.Sprintf("Error in server: %s", dst.String()), "err", err)
				//return err
			} else {
				defer c.Close()
			}
		}
		// Now start the clients for srcDest pair
		for i, conn := range pairs {
			testInfo := fmt.Sprintf("%v -> %v (%v/%v)", conn.Src.IA, conn.Dst.IA, i+1, len(pairs))
			log.Info(fmt.Sprintf("Test %v: %s", in.Name(), testInfo))
			if err := sintegration.RunClient(in, conn, 10*time.Second); err != nil {
				log.Error(fmt.Sprintf("Error in client: %s", testInfo), "err", err)
				return err
			}
		}
		return nil
	})
}

