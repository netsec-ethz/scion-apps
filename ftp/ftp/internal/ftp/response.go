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

import (
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/internal/socket"
)

// Response represents a data-connection
type Response struct {
	conn   socket.DataSocket
	c      *ServerConn
	closed bool
}

// Read implements the io.Reader interface on a FTP data connection.
func (r *Response) Read(buf []byte) (int, error) {
	return r.conn.Read(buf)
}

// Close implements the io.Closer interface on a FTP data connection.
// After the first call, Close will do nothing and return nil.
func (r *Response) Close() error {
	if r.closed {
		return nil
	}
	err := r.conn.Close()
	_, _, err2 := r.c.conn.ReadResponse(StatusClosingDataConnection)
	if err2 != nil {
		err = err2
	}
	r.closed = true
	return err
}

// SetDeadline sets the deadlines associated with the connection.
func (r *Response) SetDeadline(t time.Time) error {
	return r.conn.SetDeadline(t)
}
