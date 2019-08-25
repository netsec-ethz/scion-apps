package striping

import (
	"github.com/elwin/transmit2/queue"
)

type SegmentQueue struct {
	internal    queue.Queue
	offset      uint64
	openStreams int
}

var _ queue.Sortable = &Item{}

type Item struct {
	Segment
}

func (item *Item) Less(b queue.Sortable) bool {
	return item.OffsetCount < b.(*Item).OffsetCount
}

func (q *SegmentQueue) Push(segment *Segment) {
	q.internal.Push(segment)
}

func (q *SegmentQueue) Pop() *Segment {
	return q.internal.Pop().(*Segment)
}

func (q *SegmentQueue) Peek() *Segment {
	return q.internal.Peek().(*Segment)
}

func (q *SegmentQueue) Len() int {
	return q.internal.Len()
}

func NewSegmentQueue(workers int) *SegmentQueue {
	return &SegmentQueue{
		internal:    queue.NewQueue(),
		offset:      0,
		openStreams: workers,
	}
}

func (q *SegmentQueue) PushChannel() (chan<- *Segment, <-chan *Segment) {
	// Make buffered channels 4 times as large as the number of streams
	push := make(chan *Segment, q.openStreams*4)
	pop := make(chan *Segment, q.openStreams*4)
	go func() {
		for {
			// Has received everything
			if q.openStreams == 0 && q.Len() == 0 {
				close(push)
				close(pop)
				return
			}

			// Empty packet
			if q.Len() > 0 && q.Peek().ByteCount == 0 {
				q.Pop()
			} else if q.Len() > 0 && q.offset == q.Peek().OffsetCount {
				select {
				// Do not want to op if case not selected
				case pop <- q.Peek():
					sent := q.Pop()
					q.offset += sent.ByteCount
				case next := <-push:
					q.handleSegment(next)
				}
			} else if q.openStreams > 0 {
				q.handleSegment(<-push)
			}
		}
	}()

	return push, pop
}

func (q *SegmentQueue) handleSegment(next *Segment) {
	if next.ContainsFlag(BlockFlagEndOfData) {
		q.openStreams--
	}
	q.Push(next)
}
