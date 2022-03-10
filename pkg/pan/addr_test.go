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

package pan_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

func TestUDPAddrIsValid(t *testing.T) {
	ia := pan.MustParseIA("1-ff00:0:0")
	iaWildcard := pan.MustParseIA("1-0")
	ip := netaddr.MustParseIP("127.0.0.1")
	cases := []struct {
		addr    pan.UDPAddr
		isValid bool
	}{
		{pan.UDPAddr{}, false},
		{pan.UDPAddr{IA: ia}, false},
		{pan.UDPAddr{IP: ip}, false},
		{pan.UDPAddr{IA: ia, IP: ip}, true},
		{pan.UDPAddr{IA: iaWildcard, IP: ip}, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.isValid, c.addr.IsValid(), fmt.Sprintf("%s IsValid?", c.addr))
		// Port does not affect IsValid
		withPort := c.addr.WithPort(8888)
		assert.Equal(t, c.isValid, withPort.IsValid(), fmt.Sprintf("%s IsValid?", withPort))
	}
}
