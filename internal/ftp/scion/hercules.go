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

package scion

import (
	"crypto/tls"
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

func AllocateUDPPort(remoteAddress string) (uint16, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"scionftp"},
	}

	session, err := appquic.Dial(remoteAddress, tlsConfig, nil)
	if err != nil {
		return 0, fmt.Errorf("unable to dial %s: %s", remoteAddress, err)
	}

	stream, err := session.OpenStream()
	if err != nil {
		return 0, fmt.Errorf("unable to open stream: %s", err)
	}

	_, port, err := ParseCompleteAddress(session.LocalAddr().String())
	if err != nil {
		return 0, err
	}

	err = sendHandshake(stream)
	if err != nil {
		return 0, err
	}

	err = stream.Close()
	if err != nil {
		return 0, err
	}

	return port, nil
}
