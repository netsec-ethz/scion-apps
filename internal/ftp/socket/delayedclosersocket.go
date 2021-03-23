// Copyright 2021 ETH Zurich
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
	"io"
	"net"
	"time"
)

// DelayedCloserSocket is used to postpone calling Close() on an underlying IO that provides buffers we can't immediately free up.
type DelayedCloserSocket struct {
	net.Conn
	io.Closer
	time.Duration
}

func (s DelayedCloserSocket) Close() error {
	go func() {
		time.Sleep(s.Duration)
		_ = s.Closer.Close()
	}()
	return s.Conn.Close()
}
