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
	"fmt"
	"strings"

	"github.com/scionproto/scion/private/path/fabridquery"
)

var (
	AvailablePreferencePolicies = []string{"latency", "bandwidth", "hops", "mtu"}
	preferencePolicies          = map[string]Policy{
		"latency":   LowestLatency{},
		"bandwidth": HighestBandwidth{},
		"hops":      LeastHops{},
		"mtu":       HighestMTU{},
	}
)

// PolicyFromCommandline is a utilty function to create a path policy
// from command line options.
//
// The intent of this function is to help providing a somewhat
// consistent CLI interface for applications using this library,
// without enforcing the use of a specific command line flag
// library.
//
// The options should be presented to the user as:
//   - a flag --interactive
//   - an option --preference <preference>, sorting order for paths.
//     Comma-separated list of available sorting options.
//   - an option --sequence <sequence>, describing a hop-predicate sequence filter
func PolicyFromCommandline(sequence string, preference string, interactive bool, fabridQuery string) (Policy, error) {
	chain := PolicyChain{}
	if sequence != "" {
		seq, err := NewSequence(sequence)
		if err != nil {
			return nil, err
		}
		chain = append(chain, seq)
	}
	if preference != "" {
		preferences := strings.Split(preference, ",")
		// apply in reverse order (least important first)
		for i := len(preferences) - 1; i >= 0; i-- {
			if p, ok := preferencePolicies[preferences[i]]; ok {
				chain = append(chain, p)
			} else {
				return nil, fmt.Errorf("unknown preference sorting policy '%s'", preferences[i])
			}
		}
	}
	if fabridQuery != "" {
		query, err := fabridquery.ParseFabridQuery(fabridQuery)
		if err != nil {
			return nil, err
		}
		chain = append(chain, &FabridPolicySelection{
			FabridQuery: query,
		})
	}
	if interactive {
		chain = append(chain, &InteractiveSelection{
			Prompter: CommandlinePrompter{},
		})
	}
	if len(chain) == 1 {
		return chain[0], nil
	} else {
		return chain, nil
	}
}
