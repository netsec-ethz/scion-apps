package socket

import (
	"io"

	"github.com/netsec-ethz/scion-apps/ftp/striping"
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
