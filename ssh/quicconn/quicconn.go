package quicconn

import (
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

// QuicConn is a struct wrapping a single QUIC stream into a net.Conn connection.
type QuicConn struct {
	Session quic.Session
	Stream  quic.Stream
}

// Read reads data from the connection.
// Read can be made to time out and return an Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (mc *QuicConn) Read(b []byte) (n int, err error) {
	return mc.Stream.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetWriteDeadline.
func (mc *QuicConn) Write(b []byte) (n int, err error) {
	return mc.Stream.Write(b)
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (mc *QuicConn) Close() error {
	return mc.Stream.Close()
}

// LocalAddr returns the local network address.
func (mc *QuicConn) LocalAddr() net.Addr {
	return mc.Session.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (mc *QuicConn) RemoteAddr() net.Addr {
	return mc.Session.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail with a timeout (see type Error) instead of
// blocking. The deadline applies to all future and pending
// I/O, not just the immediately following call to Read or
// Write. After a deadline has been exceeded, the connection
// can be refreshed by setting a deadline in the future.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (mc *QuicConn) SetDeadline(t time.Time) error {
	return mc.Stream.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (mc *QuicConn) SetReadDeadline(t time.Time) error {
	return mc.Stream.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (mc *QuicConn) SetWriteDeadline(t time.Time) error {
	return mc.Stream.SetWriteDeadline(t)
}
