package striping

import (
	"container/heap"
)

var _ heap.Interface = segmentHeap{}

type segmentHeap struct {
	segments *[]*Segment
}

func (sh segmentHeap) Len() int {
	return len(*sh.segments)
}

func (sh segmentHeap) Less(i, j int) bool {
	return (*sh.segments)[i].OffsetCount < (*sh.segments)[j].OffsetCount
}

func (sh segmentHeap) Swap(i, j int) {
	(*sh.segments)[i], (*sh.segments)[j] = (*sh.segments)[j], (*sh.segments)[i]
}

func (sh segmentHeap) Push(x interface{}) {
	*sh.segments = append(*sh.segments, x.(*Segment))
}

func (sh segmentHeap) Pop() interface{} {
	s := (*sh.segments)[sh.Len()-1]
	*sh.segments = (*sh.segments)[:sh.Len()-1]
	return s
}

func newSegmentHeap() segmentHeap {
	a := make([]*Segment, 0)
	return segmentHeap{
		&a,
	}
}
