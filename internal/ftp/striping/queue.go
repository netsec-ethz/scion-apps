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
	"container/heap"
)

type SegmentQueue struct {
	internal    heap.Interface
	offset      uint64
	openStreams int
}

func (q *SegmentQueue) Push(segment *Segment) {
	heap.Push(q.internal, segment)
}

func (q *SegmentQueue) Pop() *Segment {
	return heap.Pop(q.internal).(*Segment)
}

func (q *SegmentQueue) Peek() *Segment {
	return (*q.internal.(segmentHeap).segments)[0]
}

func (q *SegmentQueue) Len() int {
	return q.internal.Len()
}

func NewSegmentQueue(workers int) *SegmentQueue {
	return &SegmentQueue{
		internal:    newSegmentHeap(),
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
