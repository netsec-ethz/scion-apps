// Copyright 2018 ETH Zurich
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

package appnet

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"

	"github.com/bclicn/color"
	log "github.com/inconshreveable/log15"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

// metrics for path selection
const (
	PathAlgoDefault = iota // default algorithm
	MTU                    // metric for path with biggest MTU
	Shortest               // metric for shortest path
)

// ChoosePathInteractive presents the user a selection of paths to choose from.
// If the remote address is in the local IA, return (nil, nil), without prompting the user.
func ChoosePathInteractive(dst addr.IA) (snet.Path, error) {

	paths, err := QueryPaths(dst)
	if err != nil || len(paths) == 0 {
		return nil, err
	}

	fmt.Printf("Available paths to %v\n", dst)
	for i, path := range paths {
		fmt.Printf("[%2d] %s\n", i, fmt.Sprintf("%s", path))
	}

	var selectedPath snet.Path
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("Choose path: ")
		scanner.Scan()
		pathIndexStr := scanner.Text()
		pathIndex, err := strconv.Atoi(pathIndexStr)
		if err == nil && 0 <= pathIndex && pathIndex < len(paths) {
			selectedPath = paths[pathIndex]
			break
		}
		fmt.Printf("ERROR: Invalid path index %v, valid indices range: [0, %v]\n", pathIndex, len(paths)-1)
	}
	re := regexp.MustCompile(`\d{1,4}-([0-9a-f]{1,4}:){2}[0-9a-f]{1,4}`)
	fmt.Printf("Using path:\n %s\n", re.ReplaceAllStringFunc(fmt.Sprintf("%s", selectedPath), color.Cyan))
	return selectedPath, nil
}

// ChoosePathByMetric chooses the best path based on the metric pathAlgo
// If the remote address is in the local IA, return (nil, nil).
func ChoosePathByMetric(pathAlgo int, dst addr.IA) (snet.Path, error) {

	paths, err := QueryPaths(dst)
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	return pathSelection(paths, pathAlgo), nil
}

// SetPath is a helper function to set the path on an snet.UDPAddr
func SetPath(addr *snet.UDPAddr, path snet.Path) {
	if path == nil {
		addr.Path = nil
		addr.NextHop = nil
	} else {
		addr.Path = path.Path()
		addr.NextHop = path.OverlayNextHop()
	}
}

// SetDefaultPath sets the first path returned by a query to sciond.
// This is a no-op if if remote is in the local AS.
func SetDefaultPath(addr *snet.UDPAddr) error {
	paths, err := QueryPaths(addr.IA)
	if err != nil || len(paths) == 0 {
		return err
	}
	SetPath(addr, paths[0])
	return nil
}

// QueryPaths queries the DefNetwork's sciond PathQuerier connection for paths to addr
// If addr is in the local IA, an empty slice and no error is returned.
func QueryPaths(ia addr.IA) ([]snet.Path, error) {
	if ia == DefNetwork().IA {
		return nil, nil
	} else {
		paths, err := DefNetwork().PathQuerier.Query(context.Background(), ia)
		if err != nil || len(paths) == 0 {
			return nil, err
		}
		return paths, nil
	}
}

func pathSelection(paths []snet.Path, pathAlgo int) snet.Path {
	var selectedPath snet.Path
	var metric float64
	// A path selection algorithm consists of a simple comparison function selecting the best path according
	// to some path property and a metric function normalizing that property to a value in [0,1], where larger is better
	// Available path selection algorithms, the metric returned must be normalized between [0,1]:
	pathAlgos := map[int](func([]snet.Path) (snet.Path, float64)){
		Shortest: selectShortestPath,
		MTU:      selectLargestMTUPath,
	}
	switch pathAlgo {
	case Shortest:
		log.Debug("Path selection algorithm", "pathAlgo", "shortest")
		selectedPath, metric = pathAlgos[pathAlgo](paths)
	case MTU:
		log.Debug("Path selection algorithm", "pathAlgo", "MTU")
		selectedPath, metric = pathAlgos[pathAlgo](paths)
	default:
		// Default is to take result with best score
		for _, algo := range pathAlgos {
			cadidatePath, cadidateMetric := algo(paths)
			if cadidateMetric > metric {
				selectedPath = cadidatePath
				metric = cadidateMetric
			}
		}
	}
	log.Debug("Path selection algorithm choice", "path", fmt.Sprintf("%s", selectedPath), "score", metric)
	return selectedPath
}

func selectShortestPath(paths []snet.Path) (selectedPath snet.Path, metric float64) {
	// Selects shortest path by number of hops
	for _, path := range paths {
		if selectedPath == nil || len(path.Interfaces()) < len(selectedPath.Interfaces()) {
			selectedPath = path
		}
	}
	metricFn := func(rawMetric int) (result float64) {
		hopCount := float64(rawMetric)
		midpoint := 7.0
		result = math.Exp(-(hopCount - midpoint)) / (1 + math.Exp(-(hopCount - midpoint)))
		return result
	}
	return selectedPath, metricFn(len(selectedPath.Interfaces()))
}

func selectLargestMTUPath(paths []snet.Path) (selectedPath snet.Path, metric float64) {
	// Selects path with largest MTU
	for _, path := range paths {
		if selectedPath == nil || path.MTU() > selectedPath.MTU() {
			selectedPath = path
		}
	}
	metricFn := func(rawMetric uint16) (result float64) {
		mtu := float64(rawMetric)
		midpoint := 1500.0
		tilt := 0.004
		result = 1 / (1 + math.Exp(-tilt*(mtu-midpoint)))
		return result
	}
	return selectedPath, metricFn(selectedPath.MTU())
}
