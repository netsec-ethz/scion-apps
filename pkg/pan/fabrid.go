// Copyright 2025 ETH Zurich
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
	"github.com/scionproto/scion/pkg/snet"
	snetpath "github.com/scionproto/scion/pkg/snet/path"
	"github.com/scionproto/scion/private/path/fabridquery"
)

type FabridPolicySelection struct {
	// The Fabrid query which is used for filtering paths
	FabridQuery fabridquery.Expressions
}

// Filters out all paths that do not satisfy the specified FABRID policy
func (s *FabridPolicySelection) Filter(paths []*Path) []*Path {
	fabridPaths := []*Path{}
	for _, p := range paths {
		_, isSCIONPath := p.ForwardingPath.dataplanePath.(snetpath.SCION)
		if !isSCIONPath {
			continue
		}
		snetMetadata := snet.PathMetadata{
			Interfaces: convertInterfaces(p.Metadata.Interfaces),
			FabridInfo: p.Metadata.FabridInfo}
		hopIntfs := snetMetadata.Hops()
		ml := fabridquery.MatchList{
			SelectedPolicies: make([]*fabridquery.Policy, len(hopIntfs)),
		}
		_, pols := s.FabridQuery.Evaluate(hopIntfs, &ml)

		if !pols.Accepted() {
			continue
		}

		fabridPaths = append(fabridPaths, p)
	}
	return fabridPaths
}
