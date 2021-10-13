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
	"os/exec"
	"path"
	"regexp"
	"testing"

	"github.com/netsec-ethz/scion-apps/pkg/integration"
)

const (
	clientBin = "scion-imagefetcher"
	serverBin = "scion-imageserver"
)

func TestMain(m *testing.M) {
	integration.TestMain(m)
}

func TestIntegrationImagefetcher(t *testing.T) {
	clientCmd := integration.AppBinPath(clientBin)
	serverCmd := integration.AppBinPath(serverBin)

	// Server
	serverPort := "42002"
	serverArgs := []string{"-p", serverPort, "-d", "testdata"}

	// Sample file path
	sample := path.Join("testdata", "logo.jpg")

	// Client
	// Image fetcher output directory
	outputDir := t.TempDir()
	sampleOutput := path.Join(outputDir, "download.jpg")

	clientArgs := []string{"-s", integration.DstAddrPattern + ":" + serverPort, "-output", sampleOutput}

	in := integration.NewAppsIntegration(clientCmd, serverCmd, clientArgs, serverArgs)
	in.ClientOutMatch = func(out string) error {
		re := regexp.MustCompile(`^r[.rT]+\nDone, exiting. Total duration \d+\.\d+m?s\n$`)
		if !re.MatchString(out) {
			return fmt.Errorf("does not match regexp '%s'", re)
		}

		// The image was downloaded, compare it with the source
		cmd := exec.Command("cmp", "--verbose", sampleOutput, sample)
		// cmp exits with 0 exit status if the files are identical, and err is nil if the exit status is 0
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("comparing downloaded to ground truth: %w\ncommand:\n%s\noutput:\n%s\n",
				err, cmd, out)
		}
		return nil
	}

	iaPairs := integration.DefaultIAPairs()
	if err := in.Run(t, iaPairs); err != nil {
		t.Error(err)
	}
}
