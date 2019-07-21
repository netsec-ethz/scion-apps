package scion

import (
	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/snet"
	"net"
)

var _ net.Conn = &Connection{}

type Connection struct {
	quic.Stream
	local, remote *snet.Addr
}

func NewSQuicConnection(stream quic.Stream, local, remote *snet.Addr) *Connection {
	return &Connection{stream, local, remote}
}

func (conn *Connection) LocalAddr() net.Addr {
	return conn.local
}

func (conn *Connection) RemoteAddr() net.Addr {
	return conn.remote
}