package socket

import (
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/scion"
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
}

var _ DataSocket = &ScionSocket{}

type ScionSocket struct {
	*scion.Connection
}

func NewScionSocket(conn *scion.Connection) *ScionSocket {
	return &ScionSocket{conn}
}
