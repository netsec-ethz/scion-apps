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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseParameters(t *testing.T) {
	defaultDuration := DefaultDuration

	cases := []struct {
		name               string
		input              string
		inferedPktSize     int // input, value inferred from path MTU
		expectedDuration   time.Duration
		expectedPacketSize int
		expectedNumPackets int
		expectErr          bool
	}{
		{
			name:               "bw 1Mbps",
			input:              "1Mbps",
			inferedPktSize:     1400,
			expectedDuration:   defaultDuration,
			expectedPacketSize: 1400,
			expectedNumPackets: 267,
		},
		{
			name:               "bw 10Mbps",
			input:              "10Mbps",
			inferedPktSize:     1400,
			expectedDuration:   defaultDuration,
			expectedPacketSize: 1400,
			expectedNumPackets: 2678,
		},
		{
			name:               "1s, 1Mbps",
			input:              "1,?,?,1Mbps",
			inferedPktSize:     1400,
			expectedDuration:   time.Second,
			expectedPacketSize: 1400,
			expectedNumPackets: 89,
		},
		{
			name:               "1s, 1Mbps",
			input:              "1,?,?,1Mbps",
			inferedPktSize:     1400,
			expectedDuration:   time.Second,
			expectedPacketSize: 1400,
			expectedNumPackets: 89,
		},
		{
			name:               "computed bw",
			input:              "1,1000,1000,?",
			expectedDuration:   time.Second,
			expectedPacketSize: 1000,
			expectedNumPackets: 1000,
		},
		{
			name:               "computed bw, default pkt size",
			input:              "1,?,1000,?",
			inferedPktSize:     1000,
			expectedDuration:   time.Second,
			expectedPacketSize: 1000,
			expectedNumPackets: 1000,
		},
		{
			name:               "redundant bw",
			input:              "1,1000,1000,8Mbps",
			inferedPktSize:     1000,
			expectedDuration:   time.Second,
			expectedPacketSize: 1000,
			expectedNumPackets: 1000,
		},
		{
			name:      "conflicting bw",
			input:     "1,1000,1000,1Mbps", // actual: 8Mbps
			expectErr: true,
		},
		{
			name:               "computed packet size",
			input:              "3,?,3000,8Mbps",
			inferedPktSize:     100,
			expectedDuration:   3 * time.Second,
			expectedPacketSize: 1000,
			expectedNumPackets: 3000,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ret, err := parseBwtestParameters(c.input, int64(c.inferedPktSize))
			if c.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.expectedDuration, ret.BwtestDuration, "duration")
				assert.Equal(t, int64(c.expectedPacketSize), ret.PacketSize, "packet size")
				assert.Equal(t, int64(c.expectedNumPackets), ret.NumPackets, "num packets")
			}
		})
	}
}
