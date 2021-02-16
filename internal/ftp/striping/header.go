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

// Extended Block Header Flags
const (
	// BlockFlagEndOfDataCount uint8 = 64
	// BlockFlagSuspectErrors  uint8 = 32
	BlockFlagEndOfData uint8 = 8
	// BlockFlagSenderClosesConnection uint8 = 4

	// Deprecated: Around for legacy purposes
	// BlockFlagEndOfRecord uint8 = 128
	// Deprecated: Around for legacy purposes
	// BlockFlagRestartMarker uint8 = 16
)

// The header is sent over the data channels and indicates
// information about the following data (if any)
// See https://www.ogf.org/documents/GWD-R/GFD-R.020.pdf
// section "Extended Block Mode"
type Header struct {
	Descriptor  uint8
	ByteCount   uint64
	OffsetCount uint64
}

func NewHeader(byteCount, offsetCount uint64, flags ...uint8) *Header {
	header := Header{
		ByteCount:   byteCount,
		OffsetCount: offsetCount,
	}

	header.AddFlag(flags...)

	return &header
}

func (header *Header) ContainsFlag(flag uint8) bool {
	return header.Descriptor&flag == flag
}

func (header *Header) AddFlag(flags ...uint8) {
	for _, flag := range flags {
		header.Descriptor |= flag
	}
}

func (header *Header) GetEODCount() int {
	return int(header.OffsetCount)
}
