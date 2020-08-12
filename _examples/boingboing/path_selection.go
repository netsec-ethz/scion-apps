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

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

func parsePathIndex(index string, max int) (pathIndex uint64, err error) {
	pathIndex, err = strconv.ParseUint(index, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid choice: '%v', %v", index, err)
	}
	if int(pathIndex) > max {
		return 0, fmt.Errorf("invalid choice: '%v', valid indices range: [0, %v]", index, max)
	}
	return
}

func parsePathChoice(selection string, max int) (pathIndices []uint64, err error) {
	var pathIndex uint64

	// Split tokens
	pathIndicesStr := strings.Split(selection[:len(selection)-1], " ")
	for _, pathIndexStr := range pathIndicesStr {
		if strings.Contains(pathIndexStr, "-") {
			// Handle ranges
			pathIndexRangeBoundaries := strings.Split(pathIndexStr, "-")
			if len(pathIndexRangeBoundaries) != 2 ||
				pathIndexRangeBoundaries[0] == "" ||
				pathIndexRangeBoundaries[1] == "" {
				return nil, fmt.Errorf("invalid path range choice: '%v'", pathIndexStr)
			}

			pathIndexRangeStart, err := parsePathIndex(pathIndexRangeBoundaries[0], max)
			if err != nil {
				return nil, err
			}
			pathIndexRangeEnd, err := parsePathIndex(pathIndexRangeBoundaries[1], max)
			if err != nil {
				return nil, err
			}

			for i := pathIndexRangeStart; i <= pathIndexRangeEnd; i++ {
				pathIndices = append(pathIndices, i)
			}
		} else {
			// Handle individual entries
			pathIndex, err = parsePathIndex(pathIndexStr, max)
			if err != nil {
				return nil, err
			}
			pathIndices = append(pathIndices, pathIndex)
		}
	}
	if len(pathIndices) < 1 {
		return nil, fmt.Errorf("no path selected: '%v'", selection)
	}
	return pathIndices, nil
}

func choosePaths(dst addr.IA, interactive bool) ([]snet.Path, error) {
	var paths []snet.Path

	paths, err := appnet.QueryPaths(dst)
	if err != nil || len(paths) == 0 {
		return nil, err
	}

	if interactive {
		fmt.Printf("Available paths to %v:\t(you can select multiple paths, such as ranges like A-C and multiple space separated path like B D F-H)\n", remote.IA)
		for i, path := range paths {
			fmt.Printf("[%2d] %s\n", i, path)
		}
		reader := bufio.NewReader(os.Stdin)
		var pathIndices []uint64
		for {
			fmt.Printf("\nChoose paths: ")
			pathIndexStr, err := reader.ReadString('\n')
			if err == io.EOF {
				fmt.Println()
				os.Exit(0)
			} else if err != nil {
				LogFatal("Error reading input", "err", err)
			}
			pathIndices, err = parsePathChoice(pathIndexStr, len(paths)-1)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Invalid path selection. %v\n", err)
				continue
			}
			break
		}

		var selectedPaths []snet.Path
		for _, i := range pathIndices {
			selectedPaths = append(selectedPaths, paths[i])
		}
		paths = selectedPaths
	}

	fmt.Println("Using paths:")
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
	fmt.Println()
	return paths, nil
}
