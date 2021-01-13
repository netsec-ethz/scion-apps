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

package socket

import (
	"io"

	"github.com/netsec-ethz/scion-apps/internal/ftp/striping"
)

type ReaderSocket struct {
	sockets []DataSocket
	queue   *striping.SegmentQueue
	pop     <-chan *striping.Segment
}

var _ io.Reader = &ReaderSocket{}
var _ io.Closer = &ReaderSocket{}

func NewReadsocket(sockets []DataSocket) *ReaderSocket {
	return &ReaderSocket{
		sockets: sockets,
		queue:   striping.NewSegmentQueue(len(sockets)),
	}
}

func (s *ReaderSocket) Read(p []byte) (n int, err error) {

	if s.pop == nil {
		push, pop := s.queue.PushChannel()
		s.pop = pop

		for _, subSocket := range s.sockets {
			reader := NewReadWorker(subSocket)
			go reader.Run(push)
		}
	}

	next := <-s.pop

	// Channel has been closed -> no more segments
	if next == nil {
		return 0, io.EOF
	}

	// If copy copies less then the ByteCount we have a problem
	return copy(p, next.Data), nil

}

func (s *ReaderSocket) Close() error {
	panic("implement me")
}
