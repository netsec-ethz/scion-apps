// Copyright 2021 ETH Zurich
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

package pan

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type InteractiveSelectionType int

const (
	InteractiveSelectionTypePinned = iota
	InteractiveSelectionTypePreferred
)

// InteractiveSelection is a path policy that prompts for paths once per
// destination IA
type InteractiveSelection struct {
	Type     InteractiveSelectionType
	Prompter Prompter
	choices  map[IA][]PathFingerprint
}

func (p *InteractiveSelection) Filter(paths []*Path) []*Path {
	dstIA := paths[0].Destination
	choice, ok := p.choices[dstIA]
	if !ok {
		chosenPaths := p.Prompter.Prompt(paths, dstIA)
		choice = pathFingerprints(chosenPaths)
		if p.choices == nil {
			p.choices = make(map[IA][]PathFingerprint)
		}
		p.choices[dstIA] = choice
	}
	if p.Type == InteractiveSelectionTypePinned {
		return Pinned{choice}.Filter(paths)
	} else {
		return Preferred{choice}.Filter(paths)
	}
}

// Prompter is used by InteractiveSelection to prompt a user for path
type Prompter interface {
	Prompt(paths []*Path, remote IA) []*Path
}

var (
	// commandlinePrompterMutex asserts that only one CommandlinePrompter is prompting at
	// any time
	commandlinePrompterMutex sync.Mutex
)

// CommandlinePrompter is a Prompter for InteractiveSelection, prompting the user for textual
// path selection input on stdin/out.
type CommandlinePrompter struct{}

func (p CommandlinePrompter) Prompt(paths []*Path, remote IA) []*Path {
	commandlinePrompterMutex.Lock()
	defer commandlinePrompterMutex.Unlock()

	fmt.Printf("Paths to %v\n", remote)
	for i, path := range paths {
		fmt.Printf("[%2d] %s\n", i, path)
	}

	var pathIndices []int
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("Choose path: ")
		if !scanner.Scan() {
			return nil
		}
		var err error
		pathIndices, err = parsePathChoice(scanner.Text(), len(paths)-1)
		if err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "ERROR: Invalid path selection. %v\n", err)
	}

	var selectedPaths []*Path
	for _, i := range pathIndices {
		selectedPaths = append(selectedPaths, paths[i])
	}
	return selectedPaths
}

// TODO copied over from nesquic demo with minimal changes. Parsing should be
// improved (e.g. handle whitespace more gracefully)
func parsePathChoice(selection string, max int) (pathIndices []int, err error) {
	// Split tokens
	pathIndexStrs := strings.Fields(selection)
	for _, pathIndexStr := range pathIndexStrs {
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
			pathIndex, err := parsePathIndex(pathIndexStr, max)
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

func parsePathIndex(index string, max int) (int, error) {
	pathIndex, err := strconv.ParseUint(index, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid choice: '%v', %v", index, err)
	}
	if pathIndex > uint64(max) {
		return 0, fmt.Errorf("invalid choice: '%v', valid indices range: [0, %v]", index, max)
	}
	return int(pathIndex), nil
}
