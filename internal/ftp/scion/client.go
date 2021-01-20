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
	"encoding/binary"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/internal/ftp/socket"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"io"
)

func DialAddr(remoteAddr string, openKeepAlive bool) (*socket.ScionSocket, *socket.ScionSocket, error) {

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"scionftp"},
	}

	session, err := appquic.Dial(remoteAddr, tlsConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dial %s: %s", remoteAddr, err)
	}

	stream, err := AddStream(&session)
	if err != nil {
		return nil, nil, err
	}
	conn := socket.NewScionSocket(session, *stream)

	var kConn *socket.ScionSocket = nil
	if openKeepAlive {
		kStream, err := AddStream(&session)
		if err != nil {
			return nil, nil, err
		}
		kConn = socket.NewScionSocket(session, *kStream)
	}

	return conn, kConn, nil
}

func AddStream(session *quic.Session) (*quic.Stream, error) {
	stream, err := (*session).OpenStream()
	if err != nil {
		return nil, fmt.Errorf("unable to open stream: %s", err)
	}

	err = sendHandshake(stream)
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func sendHandshake(rw io.ReadWriter) error {

	msg := []byte{200}

	return binary.Write(rw, binary.BigEndian, msg)

	// log.Debug("Sent handshake", "msg", msg)
}
