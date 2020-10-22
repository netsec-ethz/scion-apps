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

func NewAppQuicConnection(stream quic.Stream, local, remote Address) *Connection {
	return &Connection{stream, local, remote}
}

func (conn *Connection) LocalAddr() net.Addr {
	return conn.local
}

func (conn *Connection) RemoteAddr() net.Addr {
	return conn.remote
}

func (conn *Connection) Close() error {
	return conn.Stream.Close()
}

// Contains address with readable port etc
// There should be another way to handle this,
// together with net.Addr
func (conn *Connection) LocalAddress() Address {
	return conn.local
}

func (conn *Connection) RemoteAddress() Address {
	return conn.remote
}
