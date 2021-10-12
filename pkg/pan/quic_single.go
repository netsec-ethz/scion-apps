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
	"context"
	"net"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
)

// QUICSingleStreamListener is a convenience wrapper for a quic.Listener, that
// will accept and return QUIC sessions with a single open stream. This allows to use
// quic in contexts where a net.Listener is expected.
type QUICSingleStreamListener struct {
	quic.Listener
}

func (l QUICSingleStreamListener) Accept() (net.Conn, error) {
	ctx := context.Background()
	sess, err := l.Listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return NewQUICSingleStream(sess)
}

// QUICSingleStream combines a quic.Session and a single stream on it.
// Close will close both the Stream and the Session, making it suitable
// in contexts where a net.Conn is expected.
// TODO doc
type QUICSingleStream struct {
	Session       quic.Session
	SendStream    quic.SendStream
	ReceiveStream quic.ReceiveStream
	readDeadline  time.Time
	mutex         sync.Mutex // mutex protects Stream for await in Read/Write
}

func NewQUICSingleStream(session quic.Session) (*QUICSingleStream, error) {
	sendStream, err := session.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &QUICSingleStream{
		Session:       session,
		SendStream:    sendStream,
		ReceiveStream: nil,
	}, nil
}

func (s *QUICSingleStream) LocalAddr() net.Addr {
	return s.Session.LocalAddr()
}

func (s *QUICSingleStream) RemoteAddr() net.Addr {
	return s.Session.RemoteAddr()
}

func (s *QUICSingleStream) SetDeadline(t time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.readDeadline = t
	if s.ReceiveStream != nil {
		return s.ReceiveStream.SetReadDeadline(t)
	}
	return s.SendStream.SetWriteDeadline(t)
}

func (s *QUICSingleStream) SetReadDeadline(t time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.readDeadline = t
	if s.ReceiveStream != nil {
		return s.ReceiveStream.SetReadDeadline(t)
	}
	return nil
}

func (s *QUICSingleStream) SetWriteDeadline(t time.Time) error {
	return s.SendStream.SetWriteDeadline(t)
}

func (s *QUICSingleStream) Read(p []byte) (int, error) {
	err := s.awaitReceiveStream()
	if err != nil {
		return 0, err
	}
	return s.ReceiveStream.Read(p)
}

func (s *QUICSingleStream) Write(p []byte) (int, error) {
	return s.SendStream.Write(p)
}

func (s *QUICSingleStream) awaitReceiveStream() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.ReceiveStream == nil {
		ctx := context.Background()
		if !s.readDeadline.IsZero() {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, s.readDeadline)
			defer cancel()
		}
		stream, err := s.Session.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		s.ReceiveStream = stream
		if !s.readDeadline.IsZero() {
			return s.ReceiveStream.SetReadDeadline(s.readDeadline)
		}
	}
	return nil
}

// CloseRead aborts receiving on this stream.
// It will ask the peer to stop transmitting stream data.
// This is analogous e.g. to net.TCPConn.CloseRead.
func (s *QUICSingleStream) CloseRead() error {
	s.ReceiveStream.CancelRead(quic.StreamErrorCode(0x0))
	return nil
}

// CloseWrite closes the stream for writing.
// This is analogous e.g. to net.TCPConn.CloseWrite
func (s *QUICSingleStream) CloseWrite() error {
	return s.SendStream.Close()
}

func (s *QUICSingleStream) Close() error {
	s.SendStream.Close()
	return s.Session.CloseWithError(0x0, "")
}
