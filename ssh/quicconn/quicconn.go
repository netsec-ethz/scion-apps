package quicconn

import (
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

type QuicConn struct {
	Session quic.Session
	Stream  quic.Stream
}

func (mc *QuicConn) Read(b []byte) (n int, err error) {
	return mc.Stream.Read(b)
}

func (mc *QuicConn) Write(b []byte) (n int, err error) {
	return mc.Stream.Write(b)
}

func (mc *QuicConn) Close() error {
	return mc.Stream.Close()
}

func (mc *QuicConn) LocalAddr() net.Addr {
	return mc.Session.LocalAddr()
}

func (mc *QuicConn) RemoteAddr() net.Addr {
	return mc.Session.RemoteAddr()
}

func (mc *QuicConn) SetDeadline(t time.Time) error {
	return mc.Stream.SetDeadline(t)
}

func (mc *QuicConn) SetReadDeadline(t time.Time) error {
	return mc.Stream.SetReadDeadline(t)
}

func (mc *QuicConn) SetWriteDeadline(t time.Time) error {
	return mc.Stream.SetWriteDeadline(t)
}
