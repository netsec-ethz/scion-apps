package socket

import (
	"fmt"
	"io"

	"github.com/elwin/transmit2/striping"
)

type ReaderSocket struct {
	sockets []DataSocket
	queue   *striping.SegmentQueue
	pop     <-chan []byte
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

	// If copy copies less then the length of next we have a problem.
	// Since this actually happens with large block sizes (> 8192)
	// we mitigate this problem by having the queue make sure to pass in
	// smaller slices. If we still copy less than the length of next
	// we have an actual problem.
	copied := copy(p, next)
	if copied < len(next) {
		return copied, fmt.Errorf("copied less to p (%d bytes) than expected (%d)", copied, len(next))
	}

	return copied, nil

}

func (s *ReaderSocket) Close() error {
	panic("implement me")
}
