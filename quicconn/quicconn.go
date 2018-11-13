package quicconn

import (
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

// QuicConn wraps quic.Session and quic.Stream to implement net.Conn
type QuicConn struct {
	Session quic.Session
	Stream  quic.Stream
}

func (qc *QuicConn) Read(b []byte) (n int, err error) {
	return qc.Stream.Read(b)
}

func (qc *QuicConn) Write(b []byte) (n int, err error) {
	return qc.Stream.Write(b)
}

func (qc *QuicConn) Close() error {
	return qc.Stream.Close()
}

func (qc *QuicConn) LocalAddr() net.Addr {
	return qc.Session.LocalAddr()
}

func (qc *QuicConn) RemoteAddr() net.Addr {
	return qc.Session.RemoteAddr()
}

func (qc *QuicConn) SetDeadline(t time.Time) error {
	return qc.Stream.SetDeadline(t)
}

func (qc *QuicConn) SetReadDeadline(t time.Time) error {
	return qc.Stream.SetReadDeadline(t)
}

func (qc *QuicConn) SetWriteDeadline(t time.Time) error {
	return qc.Stream.SetWriteDeadline(t)
}
