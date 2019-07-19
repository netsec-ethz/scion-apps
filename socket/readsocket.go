package socket

import (
	"io"
	"time"

	"github.com/elwin/transmit2/striping"
)

type ReaderSocket struct {
	sockets          []DataSocket
	queue            *striping.SegmentQueue
	dispatchedReader bool
	written          uint64
	done             int
}

var _ io.Reader = &ReaderSocket{}
var _ io.Closer = &ReaderSocket{}

func NewReadsocket(sockets []DataSocket) *ReaderSocket {
	return &ReaderSocket{
		sockets: sockets,
		queue:   striping.NewSegmentQueue(),
	}
}

func (s *ReaderSocket) Read(p []byte) (n int, err error) {

	if !s.dispatchedReader {
		s.dispatchedReader = true
		s.dispatchReader()
	}

	if s.finished() {
		return 0, io.EOF
	}

	for s.queue.Len() == 0 ||
		s.queue.Peek().OffsetCount > s.written {
		// Wait until there is a suitable segment
		time.Sleep(time.Millisecond * 10)
	}

	next := s.queue.Pop()
	s.written += next.ByteCount

	if next.ContainsFlag(striping.BlockFlagEndOfData) {
		s.done++
	}

	// If copy copies less then the ByteCount we have a problem
	return copy(p, next.Data), nil

}

// Potential race condition?
// No, because the reader push segments on the queue
// before they increase the finished count
func (s *ReaderSocket) finished() bool {
	return s.done == len(s.sockets) &&
		s.queue.Len() == 0
}

func (s *ReaderSocket) Close() error {
	panic("implement me")
}

func (s *ReaderSocket) dispatchReader() {
	for _, subSocket := range s.sockets {
		reader := NewReadWorker(s.queue, subSocket)
		go reader.Run()
	}
}
