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

package quicutil

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

var (
	// SingleStreamProto is a quic application layer protocol that transports
	// an opaque, bi-directional data stream using quic. This is intended as a
	// drop-in replacement for TCP.
	//
	// This is more fiddly than perhaps expected because quic-go does not _currently_
	// have any API that allows to wait until the send buffer is drained and save
	// shutdown (of the UDP socket, or the application) is possible.
	// See https://github.com/lucas-clemente/quic-go/issues/3291.
	// TODO: simplify this once possible (as a protocol breaking change)
	//
	// The "protocol" is:
	//  - each peer opens a unidirectional stream for sending data
	//  - once the data was read in full (i.e. the FIN frame was received, EOF
	//    condition on the stream), we explicitly signal this to the peer.
	//    The signal is opening and directly closing a second unidirectional
	//    stream.
	SingleStreamProto = "qs"
)

// SingleStreamListener is a wrapper for a quic.Listener, returning
// SingleStream connections from Accept. This allows to use quic in contexts
// where a (TCP-)net.Listener is expected.
type SingleStreamListener struct {
	pan.QUICListener
}

func (l SingleStreamListener) Accept() (net.Conn, error) {
	ctx := context.Background()
	connection, err := l.QUICListener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return NewSingleStream(connection)
}

// SingleStream implements an opaque, bi-directional data stream using QUIC,
// intending to be a drop-in replacement for TCP.
// A SingleStream is either created by
//
//   - on the client side: quic.Dial and then immediately NewSingleStream(sess)
//     with the obtained connection
//   - on the listener side: quic.Listener wrapped in SingleStreamListener, which
//     returns SingleStream from Accept.
type SingleStream struct {
	Connection    quic.Connection
	sendStream    quic.SendStream
	receiveStream quic.ReceiveStream
	readDeadline  time.Time
	mutex         sync.Mutex // mutex protects receiveStream for await
	onceOK        sync.Once
}

func NewSingleStream(connection quic.Connection) (*SingleStream, error) {
	sendStream, err := connection.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &SingleStream{
		Connection:    connection,
		sendStream:    sendStream,
		receiveStream: nil,
	}, nil
}

func (s *SingleStream) LocalAddr() net.Addr {
	return s.Connection.LocalAddr()
}

func (s *SingleStream) GetPath() *pan.Path {
	if s.Connection == nil {
		// XXX(JordiSubira): To be refactored when proper support
		// for retrieving path information.
		return nil
	}
	quicSession, ok := s.Connection.(*pan.QUICSession)
	if !ok {
		// XXX(JordiSubira): To be refactored when proper support
		// for retrieving path information.
		return nil
	}
	return quicSession.Conn.GetPath()
}

func (s *SingleStream) RemoteAddr() net.Addr {
	return s.Connection.RemoteAddr()
}

func (s *SingleStream) SetDeadline(t time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.readDeadline = t
	if s.receiveStream != nil {
		return s.receiveStream.SetReadDeadline(t)
	}
	return s.sendStream.SetWriteDeadline(t)
}

func (s *SingleStream) SetReadDeadline(t time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.readDeadline = t
	if s.receiveStream != nil {
		return s.receiveStream.SetReadDeadline(t)
	}
	return nil
}

func (s *SingleStream) SetWriteDeadline(t time.Time) error {
	return s.sendStream.SetWriteDeadline(t)
}

func (s *SingleStream) Read(p []byte) (int, error) {
	err := s.awaitReceiveStream()
	if err != nil {
		return 0, err
	}
	n, err := s.receiveStream.Read(p)
	if errors.Is(err, io.EOF) || (n == 0 && err != nil) {
		s.sendOKSignal()
	}
	return n, err
}

func (s *SingleStream) Write(p []byte) (int, error) {
	return s.sendStream.Write(p)
}

func (s *SingleStream) awaitReceiveStream() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.receiveStream == nil {
		ctx := context.Background()
		if !s.readDeadline.IsZero() {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, s.readDeadline)
			defer cancel()
		}
		stream, err := s.Connection.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		s.receiveStream = stream
		if !s.readDeadline.IsZero() {
			return s.receiveStream.SetReadDeadline(s.readDeadline)
		}
	}
	return nil
}

func (s *SingleStream) sendOKSignal() {
	s.onceOK.Do(func() {
		okSignal, err := s.Connection.OpenUniStream()
		if err != nil {
			return // otherwise ignore error here, what could we do?
		}
		okSignal.Close() // only send FIN, nothing else
	})
}

func (s *SingleStream) awaitOKSignal(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// ensure we've accepted the actual receive stream first.
	if s.receiveStream == nil {
		stream, err := s.Connection.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		s.receiveStream = stream
	}

	_, err := s.Connection.AcceptUniStream(ctx)
	// We can ignore data arriving in on this stream -- we expect
	// a single FIN, so a readeading will immediately give EOF.
	// If that's not what we get, guess we can ignore it.
	return err
}

// CloseRead aborts receiving on this stream.
// It will ask the peer to stop transmitting stream data.
// This is analogous e.g. to net.TCPConn.CloseRead.
func (s *SingleStream) CloseRead() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.receiveStream != nil {
		s.receiveStream.CancelRead(quic.StreamErrorCode(0x0))
	}
	s.sendOKSignal()
	return nil
}

// CloseWrite closes the stream for writing.
// This is analogous e.g. to net.TCPConn.CloseWrite
func (s *SingleStream) CloseWrite() error {
	return s.sendStream.Close()
}

func (s *SingleStream) CloseSync(ctx context.Context) error {
	// Close sendStream, i.e. send FIN, so peer will notice EOF eventually,
	// after which it will send the OK-signal.
	// This also ensures that the peers first AcceptUniStream unblocks, in case
	// no data was written.
	_ = s.CloseWrite()
	// Close receiveStream -- we won't be reading from this anymore --
	// and send the OK-signal if we haven't already. This allows the peer to
	// also shutdown properly.
	_ = s.CloseRead()
	// Await the OK-signal
	if err := s.awaitOKSignal(ctx); err != nil {
		return s.Connection.CloseWithError(0x101, "shutdown error")
	}
	return s.Connection.CloseWithError(0x0, "ok")
}

func (s *SingleStream) Close() error {
	ctx := context.Background()
	// Block until read deadline -- a bit arbitrary, but ok?
	if !s.readDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, s.readDeadline)
		defer cancel()
	}
	return s.CloseSync(ctx)
}
