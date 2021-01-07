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

package socket

import (
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/internal/scion"
)

// DataSocket describes a data socket is used to send non-control data between the scionftp and
// server.
type DataSocket interface {
	// the standard io.Reader interface
	Read(p []byte) (n int, err error)

	// the standard io.ReaderFrom interface
	// ReadFrom(r io.Reader) (int64, error)

	// the standard io.Writer interface
	Write(p []byte) (n int, err error)

	// the standard io.Closer interface
	Close() error

	// Set deadline associated with connection (scionftp)
	SetDeadline(t time.Time) error

	LocalAddress() scion.Address
	RemoteAddress() scion.Address
}

var _ DataSocket = &ScionSocket{}

type ScionSocket struct {
	*scion.Connection
}

func NewScionSocket(conn *scion.Connection) *ScionSocket {
	return &ScionSocket{conn}
}
