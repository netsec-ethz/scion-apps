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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrNoPathTo(t *testing.T) {
	err := errNoPathTo(IA{I: 1, A: 0xff00_0000_0001})
	assert.Equal(t, err.Error(), "no path to 1-ff00:0:1")
	assert.True(t, errors.Is(err, ErrNoPath))
}
