package socket

import (
	"context"
	"io"
	"sync"

	"github.com/netsec-ethz/scion-apps/ftp/striping"
)

type WriterSocket struct {
	sockets           []DataSocket
	maxLength         int
	segmentChannel    chan *striping.Segment
	wg                *sync.WaitGroup
	cancel            context.CancelFunc
	written           int
	dispatchedWriters bool
}

var _ io.Writer = &WriterSocket{}
var _ io.Closer = &WriterSocket{}

func NewWriterSocket(sockets []DataSocket, maxLength int) *WriterSocket {
	return &WriterSocket{
		sockets:        sockets,
		maxLength:      maxLength,
		segmentChannel: make(chan *striping.Segment),
		wg:             &sync.WaitGroup{},
	}
}

// Will dispatch workers if required and write on
// the allocated stream. After writing it is necessary
// to call FinishAndWait() to make sure that everything is sent
func (s *WriterSocket) Write(p []byte) (n int, err error) {
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

		s.segmentChannel <- striping.NewSegment(data, s.written)

		s.written += to - cur

		cur = to
	}
}

// Should only be called when all segments have been dispatchedReader,
// that is, segmentChannel should be empty
func (s *WriterSocket) FinishAndWait() {
	// Wait until all writers have finished
	if !s.dispatchedWriters {
		return
	}

	// Tell all workers to send EOD next
	s.cancel()
	s.wg.Wait()

	s.dispatchedWriters = false
}

func (s *WriterSocket) dispatchWriter() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.TODO())

	for _, socket := range s.sockets {
		s.wg.Add(1)
		worker := NewWriteWorker(ctx, s.wg, s.segmentChannel, socket)
		go worker.Run()
	}

	return cancel
}

// Closing the WriterSocket blocks until until all
// children have finished sending and then closes
// all sub-sockets
func (s *WriterSocket) Close() error {

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
