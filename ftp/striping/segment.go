package striping

import "github.com/netsec-ethz/scion-apps/ftp/queue"

type Segment struct {
	*Header
	Data []byte
}

func (a *Segment) Less(b queue.Sortable) bool {
	return a.OffsetCount < b.(*Segment).OffsetCount
}

func NewSegment(data []byte, offset int, flags ...uint8) *Segment {

	return &Segment{
		NewHeader(uint64(len(data)), uint64(offset), flags...),
		data,
	}

}

func NewSegmentWithHeader(header *Header, data []byte) *Segment {
	return &Segment{
		header,
		data,
	}
}
