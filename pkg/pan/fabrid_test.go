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
	"testing"

	"github.com/scionproto/scion/pkg/experimental/fabrid"
	"github.com/scionproto/scion/pkg/slayers/path"
	"github.com/scionproto/scion/pkg/slayers/path/scion"
	"github.com/scionproto/scion/pkg/snet"
	snetpath "github.com/scionproto/scion/pkg/snet/path"
	"github.com/scionproto/scion/private/path/fabridquery"
	"github.com/stretchr/testify/assert"
)

func TestFabridFilter(t *testing.T) {
	scionDecoded := &scion.Decoded{
		Base: scion.Base{
			PathMeta: scion.MetaHdr{
				CurrHF: 0,
				SegLen: [3]uint8{3, 0, 0},
			},
			NumINF:  1,
			NumHops: 3,
		},
		InfoFields: []path.InfoField{
			{
				SegID:   1,
				ConsDir: true,
			},
		},
		HopFields: []path.HopField{
			{ConsIngress: 0, ConsEgress: 1},
			{ConsIngress: 2, ConsEgress: 5},
			{ConsIngress: 3, ConsEgress: 0},
		},
	}
	raw, err := scionDecoded.ToRaw()
	assert.NoError(t, err)
	p := &Path{
		Source:      MustParseIA("1-ff00:0:1"),
		Destination: MustParseIA("1-ff00:0:3"),
		ForwardingPath: ForwardingPath{
			dataplanePath: snetpath.SCION{
				Raw: raw.Raw,
			},
		},
		Metadata: &PathMetadata{
			FabridInfo: []snet.FabridInfo{
				{
					Enabled: true,
					Policies: []*fabrid.Policy{
						{
							IsLocal:    false,
							Identifier: 55,
							Index:      fabrid.PolicyID(11),
						},
						{
							IsLocal:    false,
							Identifier: 33,
							Index:      fabrid.PolicyID(12),
						},
					},
				},
				{
					Enabled: true,
					Policies: []*fabrid.Policy{
						{
							IsLocal:    false,
							Identifier: 55,
							Index:      fabrid.PolicyID(22),
						},
						{
							IsLocal:    false,
							Identifier: 33,
							Index:      fabrid.PolicyID(23),
						},
					},
				},
				{
					Enabled: true,
					Policies: []*fabrid.Policy{
						{
							IsLocal:    false,
							Identifier: 55,
							Index:      fabrid.PolicyID(33),
						},
					},
				},
			},
			Interfaces: []PathInterface{
				{
					IA:   MustParseIA("1-ff00:0:1"),
					IfID: 1,
				},
				{
					IA:   MustParseIA("1-ff00:0:2"),
					IfID: 2,
				},
				{
					IA:   MustParseIA("1-ff00:0:3"),
					IfID: 5,
				},
				{
					IA:   MustParseIA("1-ff00:0:3"),
					IfID: 3,
				},
			},
		},
	}
	type testcase struct {
		name          string
		query         string
		expectedPaths int
	}
	tests := []testcase{
		{
			name:          "query to take any fabrid path",
			query:         "0-0#0,0@0",
			expectedPaths: 1,
		},
		{
			name:          "query to take global policy 55 if available",
			query:         "0-0#0,0@G55",
			expectedPaths: 1,
		},
		{
			name:          "query to reject the path if global policy 55 is not available",
			query:         "0-0#0,0@G55+0-0#0,0@REJECT",
			expectedPaths: 1,
		},
		{
			name:          "query to reject the path if global policy 33 is not available",
			query:         "0-0#0,0@G33+0-0#0,0@REJECT",
			expectedPaths: 0,
		},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			q, err := fabridquery.ParseFabridQuery(c.query)
			assert.NoError(t, err)
			f := FabridPolicySelection{
				FabridQuery: q,
			}
			res := f.Filter([]*Path{p})
			assert.Len(t, res, c.expectedPaths)
		})
	}
}
