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
	"testing"
)

func TestSplitHostPort(t *testing.T) {
	type testCase struct {
		input string
		host  string
		port  string
		err   bool
	}
	cases := []testCase{
		{"1-ff00:0:0,[1.1.1.1]:80", "1-ff00:0:0,[1.1.1.1]", "80", false},
		{"1-ff00:0:0,1.1.1.1:80", "1-ff00:0:0,1.1.1.1", "80", false},
		{"1-ff00:0:0,[::]:80", "1-ff00:0:0,[::]", "80", false},
		{"foo:80", "foo", "80", false},
		{"www.example.com:666", "www.example.com", "666", false},
		{"1-ff00:0:0,0:0:0:80", "", "", true},
		{":foo:666", "", "", true},
		{"1-ff00:0:0,[1.1.1.1]", "", "", true},
		{"1-ff00:0:0,1.1.1.1", "", "", true},
		{"1-ff00:0:0,[::]", "", "", true},
		{"foo", "", "", true},
	}
	for _, c := range cases {
		host, port, err := SplitHostPort(c.input)
		if err != nil && !c.err {
			t.Errorf("Failed, but shouldn't have. input: %s, error: %s", c.input, err)
		} else if err == nil && c.err {
			t.Errorf("Did not fail, but should have. input: %s, host: %s, port: %s", c.input, host, port)
		} else if err != nil {
			if host != c.host || port != c.port {
				t.Errorf("Bad result. input: %s, host: %s, port: %s", c.input, host, port)
			}
		}
	}
}
