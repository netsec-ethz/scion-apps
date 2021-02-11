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
	"context"
	"github.com/netsec-ethz/scion-apps/internal/ftp/socket"
	"io"
	"sync"
)

type writerSocket struct {
	sockets           []socket.DataSocket
	maxLength         int
	segmentChannel    chan *Segment
	wg                *sync.WaitGroup
	cancel            context.CancelFunc
	written           int
	dispatchedWriters bool
}

var _ io.Writer = &writerSocket{}
var _ io.Closer = &writerSocket{}

func newWriterSocket(sockets []socket.DataSocket, maxLength int) *writerSocket {
	return &writerSocket{
		sockets:        sockets,
		maxLength:      maxLength,
		segmentChannel: make(chan *Segment),
		wg:             &sync.WaitGroup{},
	}
}

// Will dispatch workers if required and write on
// the allocated stream. After writing it is necessary
// to call FinishAndWait() to make sure that everything is sent
func (s *writerSocket) Write(p []byte) (n int, err error) {
	if !s.dispatchedWriters {
		s.dispatchedWriters = true
		s.cancel = s.dispatchWriter()
	}

	cur := 0

	for {
		if cur == len(p) {
			return cur, nil
		}

		to := cur + s.maxLength
		if to > len(p) {
			to = len(p)
		}

		data := make([]byte, to-cur)
		copy(data, p[cur:to])

		s.segmentChannel <- NewSegment(data, s.written)

		s.written += to - cur

		cur = to
	}
}

// Should only be called when all segments have been dispatchedReader,
// that is, segmentChannel should be empty
func (s *writerSocket) FinishAndWait() {
	// Wait until all writers have finished
	if !s.dispatchedWriters {
		return
	}

	// Tell all workers to send EOD next
	s.cancel()
	s.wg.Wait()

	s.dispatchedWriters = false
}

func (s *writerSocket) dispatchWriter() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.TODO())

	for _, socket := range s.sockets {
		s.wg.Add(1)
		worker := newWriteWorker(ctx, s.wg, s.segmentChannel, socket)
		go worker.Run()
	}

	return cancel
}

// Closing the writerSocket blocks until until all
// children have finished sending and then closes
// all sub-sockets
func (s *writerSocket) Close() error {

	s.FinishAndWait()

	var errs []error

	for i := range s.sockets {
		err := s.sockets[i].Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	} else {
		return errs[0]
	}
}
