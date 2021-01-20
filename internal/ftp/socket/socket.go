// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Copyright 2020 ETH Zurich modifications to add support for SCION
package socket

import (
	"github.com/lucas-clemente/quic-go"
	"net"
	"time"
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

	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

var _ DataSocket = &ScionSocket{}

type ScionSocket struct {
	quic.Session
	quic.Stream
}

func NewScionSocket(sess quic.Session, stream quic.Stream) *ScionSocket {
	return &ScionSocket{sess, stream}
}
