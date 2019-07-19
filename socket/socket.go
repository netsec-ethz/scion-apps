package socket

import (
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/elwin/transmit2/logger"
)

// DataSocket describes a data socket is used to send non-control data between the client and
// server.
type DataSocket interface {

	// For server: local hostname and port
	Host() string
	Port() int

	// the standard io.Reader interface
	Read(p []byte) (n int, err error)

	// the standard io.ReaderFrom interface
	// ReadFrom(r io.Reader) (int64, error)

	// the standard io.Writer interface
	Write(p []byte) (n int, err error)

	// the standard io.Closer interface
	Close() error

	// Set deadline associated with connection (client)
	SetDeadline(t time.Time) error
}

var _ DataSocket = &ftpPassiveSocket{}

type ftpPassiveSocket struct {
	conn   net.Conn
	port   int
	host   string
	logger logger.Logger
	lock   sync.Mutex // protects conn and err
	err    error
}

// Detect if an error is "bind: address already in use"
//
// Originally from https://stackoverflow.com/a/52152912/164234
func isErrorAddressAlreadyInUse(err error) bool {
	errOpError, ok := err.(*net.OpError)
	if !ok {
		return false
	}
	errSyscallError, ok := errOpError.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	errErrno, ok := errSyscallError.Err.(syscall.Errno)
	if !ok {
		return false
	}
	if errErrno == syscall.EADDRINUSE {
		return true
	}
	const WSAEADDRINUSE = 10048
	if runtime.GOOS == "windows" && errErrno == WSAEADDRINUSE {
		return true
	}
	return false
}

func NewActiveSocket(addr string, logger logger.Logger) (DataSocket, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	socket := new(ftpPassiveSocket)
	socket.conn = conn
	socket.logger = logger

	socket.host = "clienthost"
	socket.port = 9999

	return socket, nil
}

func NewPassiveSocket(host string, port func() int, logger logger.Logger, sessionID string) (DataSocket, error) {
	socket := new(ftpPassiveSocket)
	socket.logger = logger
	socket.host = host
	const retries = 10
	var err error
	for i := 1; i <= retries; i++ {
		socket.port = port()
		err = socket.GoListenAndServe(sessionID)
		if err != nil && socket.port != 0 && isErrorAddressAlreadyInUse(err) {
			// choose a different port on error already in use
			continue
		}
		break
	}
	return socket, err
}

func (socket *ftpPassiveSocket) Host() string {
	return socket.host
}

func (socket *ftpPassiveSocket) Port() int {
	return socket.port
}

func (socket *ftpPassiveSocket) Read(p []byte) (n int, err error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}
	return socket.conn.Read(p)
}

func (socket *ftpPassiveSocket) ReadFrom(r io.Reader) (int64, error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}

	// For normal TCPConn, this will use sendfile syscall; if not,
	// it will just downgrade to normal read/write procedure
	return io.Copy(socket.conn, r)
}

func (socket *ftpPassiveSocket) Write(p []byte) (n int, err error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}
	return socket.conn.Write(p)
}

func (socket *ftpPassiveSocket) SetDeadline(t time.Time) error {
	return socket.conn.SetDeadline(t)
}

func (socket *ftpPassiveSocket) Close() error {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.conn != nil {
		return socket.conn.Close()
	}
	return nil
}

func (socket *ftpPassiveSocket) GoListenAndServe(sessionID string) (err error) {
	laddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("", strconv.Itoa(socket.port)))
	if err != nil {
		socket.logger.Print(sessionID, err)
		return
	}

	var tcplistener *net.TCPListener
	tcplistener, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		socket.logger.Print(sessionID, err)
		return
	}

	// The timeout, for a remote client to establish connection
	// with a PASV style data connection.
	const acceptTimeout = 60 * time.Second
	err = tcplistener.SetDeadline(time.Now().Add(acceptTimeout))
	if err != nil {
		socket.logger.Print(sessionID, err)
		return
	}

	var listener net.Listener = tcplistener
	add := listener.Addr()
	parts := strings.Split(add.String(), ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		socket.logger.Print(sessionID, err)
		return
	}

	socket.port = port

	socket.lock.Lock()
	go func() {
		defer socket.lock.Unlock()

		conn, err := listener.Accept()
		if err != nil {
			socket.err = err
			return
		}
		socket.err = nil
		socket.conn = conn
		_ = listener.Close()
	}()
	return nil
}
