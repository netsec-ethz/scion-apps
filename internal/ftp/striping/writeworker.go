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
	"encoding/binary"
	"net"
	"sync"

	"github.com/scionproto/scion/go/lib/log"
)

type writeWorker struct {
	ctx      context.Context
	wg       *sync.WaitGroup
	segments chan *Segment
	socket   net.Conn
}

func newWriteWorker(ctx context.Context, wg *sync.WaitGroup, segments chan *Segment, socket net.Conn) *writeWorker {
	return &writeWorker{ctx, wg, segments, socket}
}

// Writes segments until receives cancellation signal on Done()
// and sends EOD Header after that.
func (w *writeWorker) Run() {
	for {
		select {
		case segment := <-w.segments:
			err := w.writeSegment(segment)
			if err != nil {
				log.Error("Failed to write segment", "err", err)
			}

		case <-w.ctx.Done():
			eod := NewHeader(
				0, 0,
				BlockFlagEndOfData)
			err := w.writeHeader(eod)
			if err != nil {
				log.Error("Failed to write eod header", "err", err)
			}
			w.wg.Done()
			return
		}
	}
}

func (w *writeWorker) writeHeader(header *Header) error {
	return binary.Write(w.socket, binary.BigEndian, header)
}

func (w *writeWorker) writeSegment(segment *Segment) error {
	err := w.writeHeader(segment.Header)
	if err != nil {
		return err
	}

	cur := 0

	for {

		n, err := w.socket.Write(segment.Data[cur:segment.ByteCount])
		if err != nil {
			return err
		}

		cur += n

		if cur == int(segment.ByteCount) {
			break
		}

	}

	return nil
}
