package socket

import (
	"github.com/netsec-ethz/scion-apps/ftp/scion"
	"io"
	"time"
)

type Socket interface {
	io.Reader
	io.Writer
	io.Closer
}

type MultiSocket struct {
	*ReaderSocket
	*WriterSocket
	closed bool
}

func (m *MultiSocket) SetDeadline(t time.Time) error {
	panic("implement me")
}

var _ DataSocket = &MultiSocket{}

// Only the scionftp should close the socket
// Sends the closing message
func (m *MultiSocket) Close() error {
	return m.WriterSocket.Close()
}

func (m *MultiSocket) LocalAddress() scion.Address {
	return m.WriterSocket.sockets[0].LocalAddress()
}

func (m *MultiSocket) RemoteAddress() scion.Address {
	return m.WriterSocket.sockets[0].RemoteAddress()
}

var _ DataSocket = &MultiSocket{}

func NewMultiSocket(sockets []DataSocket, maxLength int) *MultiSocket {
	return &MultiSocket{
		NewReadsocket(sockets),
		NewWriterSocket(sockets, maxLength),
		false,
	}
}
