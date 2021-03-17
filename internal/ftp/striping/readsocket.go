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

package striping

import (
	"io"
	"net"
)

type readerSocket struct {
	sockets []net.Conn
	queue   *SegmentQueue
	pop     <-chan Segment
}

var _ io.Reader = &readerSocket{}
var _ io.Closer = &readerSocket{}

func newReaderSocket(sockets []net.Conn) *readerSocket {
	return &readerSocket{
		sockets: sockets,
		queue:   NewSegmentQueue(len(sockets)),
	}
}

func (s *readerSocket) Read(p []byte) (n int, err error) {

	if s.pop == nil {
		push, pop := s.queue.PushChannel()
		s.pop = pop

		for _, subSocket := range s.sockets {
			reader := newReadWorker(subSocket)
			go reader.Run(push)
		}
	}

	next := <-s.pop

	// Channel has been closed -> no more segments
	if next.Header == nil {
		return 0, io.EOF
	}

	// If copy copies less then the ByteCount we have a problem
	return copy(p, next.Data), nil

}

func (s *readerSocket) Close() error {
	panic("implement me")
}
