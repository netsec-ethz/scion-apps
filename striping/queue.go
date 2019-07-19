package striping

import (
	"fmt"

	"github.com/elwin/transmit2/queue"
)

type SegmentQueue struct {
	internal queue.Queue
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
	fmt.Println("Popping")
	return q.internal.Pop().(*Segment)
}

func (q *SegmentQueue) Peek() *Segment {
	return q.internal.Peek().(*Segment)
}

func (q *SegmentQueue) Len() int {
	return q.internal.Len()
}

func NewSegmentQueue() *SegmentQueue {
	return &SegmentQueue{queue.NewQueue()}
}
