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

package ftp

// A scanner for fields delimited by one or more whitespace characters
type scanner struct {
	bytes    []byte
	position int
}

// newScanner creates a new scanner
func newScanner(str string) *scanner {
	return &scanner{
		bytes: []byte(str),
	}
}

// NextFields returns the next `count` fields
func (s *scanner) NextFields(count int) []string {
	fields := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if field := s.Next(); field != "" {
			fields = append(fields, field)
		} else {
			break
		}
	}
	return fields
}

// Next returns the next field
func (s *scanner) Next() string {
	sLen := len(s.bytes)

	// skip trailing whitespace
	for s.position < sLen {
		if s.bytes[s.position] != ' ' {
			break
		}
		s.position++
	}

	start := s.position

	// skip non-whitespace
	for s.position < sLen {
		if s.bytes[s.position] == ' ' {
			s.position++
			return string(s.bytes[start : s.position-1])
		}
		s.position++
	}

	return string(s.bytes[start:s.position])
}

// Remaining returns the remaining string
func (s *scanner) Remaining() string {
	return string(s.bytes[s.position:len(s.bytes)])
}
