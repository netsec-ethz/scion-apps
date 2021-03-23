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

package core

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/lucas-clemente/quic-go"

	"github.com/netsec-ethz/scion-apps/internal/ftp/socket"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

type Listener struct {
	QuicListener quic.Listener
}

func ListenPort(port uint16, cert *tls.Certificate) (*Listener, error) {
	tlsConfig := &tls.Config{
		NextProtos:   []string{"scionftp"},
		Certificates: []tls.Certificate{*cert},
	}

	listener, err := appquic.ListenPort(port, tlsConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

	return &Listener{
		listener,
	}, nil
}

func (listener *Listener) Close() error {
	return listener.QuicListener.Close()
}

func (listener *Listener) Accept() (net.Conn, error) {
	return socket.Accept(listener.QuicListener)
}
