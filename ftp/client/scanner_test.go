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

package ftp

import "testing"
import "github.com/stretchr/testify/assert"

func TestScanner(t *testing.T) {
	assert := assert.New(t)

	s := newScanner("foo  bar x  y")
	assert.Equal("foo", s.Next())
	assert.Equal(" bar x  y", s.Remaining())
	assert.Equal("bar", s.Next())
	assert.Equal("x  y", s.Remaining())
	assert.Equal("x", s.Next())
	assert.Equal(" y", s.Remaining())
	assert.Equal("y", s.Next())
	assert.Equal("", s.Next())
	assert.Equal("", s.Remaining())
}

func TestScannerEmpty(t *testing.T) {
	assert := assert.New(t)

	s := newScanner("")
	assert.Equal("", s.Next())
	assert.Equal("", s.Next())
	assert.Equal("", s.Remaining())
}
