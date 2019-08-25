package scion

import (
	"net"

	"github.com/lucas-clemente/quic-go"
)

var _ net.Conn = &Connection{}

type Connection struct {
	quic.Stream
	local, remote Address
}

func NewSQuicConnection(stream quic.Stream, local, remote Address) *Connection {
	return &Connection{stream, local, remote}
}

func (conn *Connection) LocalAddr() net.Addr {
	tmp := conn.local.Addr()
	return &tmp
}

func (conn *Connection) RemoteAddr() net.Addr {
	tmp := conn.remote.Addr()
	return &tmp
}

func (conn *Connection) Close() error {
	return conn.Stream.Close()
}
