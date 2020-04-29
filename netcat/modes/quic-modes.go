// Copyright 2019 ETH Zurich
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

package modes

import (
	"io"
	golog "log"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"

	log "github.com/inconshreveable/log15"
)

type sessConn struct {
	sess   quic.Session
	stream quic.Stream
}

func (conn *sessConn) Read(b []byte) (n int, err error) {
	return conn.stream.Read(b)
}

func (conn *sessConn) Write(b []byte) (n int, err error) {
	return conn.stream.Write(b)
}

func (conn *sessConn) Close() error {
	err := conn.stream.Close()
	if err != nil {
		return err
	}

	err = conn.sess.Close()
	if err != nil {
		return err
	}

	return nil
}

// DoListenQUIC listens on a QUIC socket
func DoListenQUIC(port uint16) chan io.ReadWriteCloser {
	listener, err := appquic.ListenPort(port, nil, &quic.Config{KeepAlive: true})
	if err != nil {
		golog.Panicf("Can't listen on port %d: %v", port, err)
	}

	conns := make(chan io.ReadWriteCloser)
	go func() {
		for {
			sess, err := listener.Accept()
			if err != nil {
				log.Crit("Can't accept listener: %v", err)
				continue
			}

			stream, err := sess.AcceptStream()
			if err != nil {
				log.Crit("Can't accept stream: %v", err)
				continue
			}

			log.Info("New QUIC connection")

			conns <- &sessConn{
				sess:   sess,
				stream: stream,
			}
		}
	}()

	return conns
}

// DoDialQUIC dials with a QUIC socket
func DoDialQUIC(remoteAddr string) io.ReadWriteCloser {
	sess, err := appquic.Dial(remoteAddr, nil, &quic.Config{KeepAlive: true})
	if err != nil {
		golog.Panicf("Can't dial remote address %v: %v", remoteAddr, err)
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		golog.Panicf("Can't open stream: %v", err)
	}

	log.Debug("Connected!")

	return &sessConn{
		sess:   sess,
		stream: stream,
	}
}
