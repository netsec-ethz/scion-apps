// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Copyright 2020 ETH Zurich modifications to add support for SCION
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
