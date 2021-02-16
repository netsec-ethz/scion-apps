// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Copyright 2020 ETH Zurich modifications to add support for SCION
package socket

import (
	"github.com/lucas-clemente/quic-go"
	"net"
)

var _ net.Conn = &ScionSocket{}

type ScionSocket struct {
	quic.Session
	quic.Stream
}

func NewScionSocket(sess quic.Session, stream quic.Stream) *ScionSocket {
	return &ScionSocket{sess, stream}
}
