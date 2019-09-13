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
// limitations under the License.package main

package scionutil

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

	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

// metrics for path selection
const (
	PathAlgoDefault = iota // default algorithm
	MTU                    // metric for path with biggest MTU
	Shortest               // metric for shortest path
)

// ChoosePathInteractive presents the user a selection of paths to choose from
func ChoosePathInteractive(local, remote *snet.Addr) *sciond.PathReplyEntry {
	if snet.DefNetwork == nil {
		InitSCION(local)
	}

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA, sciond.PathReqFlags{})
	var appPaths []*spathmeta.AppPath
	var selectedPath *spathmeta.AppPath

	if len(pathSet) == 0 {
		return nil
	}

	re := regexp.MustCompile(`\d{1,4}-([0-9a-f]{1,4}:){2}[0-9a-f]{1,4}`)
	fmt.Printf("Available paths to %v\n", remote.IA)
	i := 0
	for _, path := range pathSet {
		appPaths = append(appPaths, path)
		fmt.Printf("[%2d] %s\n", i, re.ReplaceAllStringFunc(path.Entry.Path.String(), color.Cyan))
		i++
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("Choose path: ")
		scanner.Scan()
		pathIndexStr := scanner.Text()
		pathIndex, err := strconv.Atoi(pathIndexStr)
		if err == nil && 0 <= pathIndex && pathIndex < len(appPaths) {
			selectedPath = appPaths[pathIndex]
			break
		}
		fmt.Printf("ERROR: Invalid path index %v, valid indices range: [0, %v]\n", pathIndex, len(appPaths)-1)
	}
	entry := selectedPath.Entry
	fmt.Printf("Using path:\n %s\n", re.ReplaceAllStringFunc(entry.Path.String(), color.Cyan))
	return entry
}

// ChoosePathByMetric chooses the best path based on the metric pathAlgo
func ChoosePathByMetric(pathAlgo int, local, remote *snet.Addr) *sciond.PathReplyEntry {
	if snet.DefNetwork == nil {
		InitSCION(local)
	}

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA, sciond.PathReqFlags{})
	var appPaths []*spathmeta.AppPath

	i := 0
	for _, path := range pathSet {
		appPaths = append(appPaths, path)
		i++
	}

	if len(pathSet) == 0 {
		return nil
	}

	return pathSelection(pathSet, pathAlgo).Entry
}

func pathSelection(pathSet spathmeta.AppPathSet, pathAlgo int) *spathmeta.AppPath {
	var selectedPath *spathmeta.AppPath
	var metric float64
	// A path selection algorithm consists of a simple comparison function selecting the best path according
	// to some path property and a metric function normalizing that property to a value in [0,1], where larger is better
	// Available path selection algorithms, the metric returned must be normalized between [0,1]:
	pathAlgos := map[int](func(spathmeta.AppPathSet) (*spathmeta.AppPath, float64)){
		Shortest: selectShortestPath,
		MTU:      selectLargestMTUPath,
	}
	switch pathAlgo {
	case Shortest:
		log.Debug("Path selection algorithm", "pathAlgo", "shortest")
		selectedPath, metric = pathAlgos[pathAlgo](pathSet)
	case MTU:
		log.Debug("Path selection algorithm", "pathAlgo", "MTU")
		selectedPath, metric = pathAlgos[pathAlgo](pathSet)
	default:
		// Default is to take result with best score
		for _, algo := range pathAlgos {
			cadidatePath, cadidateMetric := algo(pathSet)
			if cadidateMetric > metric {
				selectedPath = cadidatePath
				metric = cadidateMetric
			}
		}
	}
	log.Debug("Path selection algorithm choice", "path", selectedPath.Entry.Path.String(), "score", metric)
	return selectedPath
}

func selectShortestPath(pathSet spathmeta.AppPathSet) (selectedPath *spathmeta.AppPath, metric float64) {
	// Selects shortest path by number of hops
	for _, appPath := range pathSet {
		if selectedPath == nil || len(appPath.Entry.Path.Interfaces) < len(selectedPath.Entry.Path.Interfaces) {
			selectedPath = appPath
		}
	}
	metricFn := func(rawMetric []sciond.PathInterface) (result float64) {
		hopCount := float64(len(rawMetric))
		midpoint := 7.0
		result = math.Exp(-(hopCount - midpoint)) / (1 + math.Exp(-(hopCount - midpoint)))
		return result
	}
	return selectedPath, metricFn(selectedPath.Entry.Path.Interfaces)
}

func selectLargestMTUPath(pathSet spathmeta.AppPathSet) (selectedPath *spathmeta.AppPath, metric float64) {
	// Selects path with largest MTU
	for _, appPath := range pathSet {
		if selectedPath == nil || appPath.Entry.Path.Mtu > selectedPath.Entry.Path.Mtu {
			selectedPath = appPath
		}
	}
	metricFn := func(rawMetric uint16) (result float64) {
		mtu := float64(rawMetric)
		midpoint := 1500.0
		tilt := 0.004
		result = 1 / (1 + math.Exp(-tilt*(mtu-midpoint)))
		return result
	}
	return selectedPath, metricFn(selectedPath.Entry.Path.Mtu)
}
