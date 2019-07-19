// Package ftp implements a FTP client as described in RFC 959.
//
// A textproto.Error is returned for errors at the protocol level.
package ftp

import (
	"context"
	"io"
	"net"
	"net/textproto"
	"time"

	"github.com/elwin/transmit2/logger"
)

// EntryType describes the different types of an Entry.
type EntryType int

// The differents types of an Entry
const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

// ServerConn represents the connection to a remote FTP server.
// A single connection only supports one in-flight data connection.
// It is not safe to be called concurrently.
type ServerConn struct {
	options *dialOptions
	conn    *textproto.Conn
	host    string

	// Server capabilities discovered at runtime
	features      map[string]string
	mlstSupported bool
	extended      bool
	logger        logger.Logger
}

// DialOption represents an option to start a new connection with Dial
type DialOption struct {
	setup func(do *dialOptions)
}

// dialOptions contains all the options set by DialOption.setup
type dialOptions struct {
	context     context.Context
	dialer      net.Dialer
	conn        net.Conn
	disableEPSV bool
	location    *time.Location
	debugOutput io.Writer
	dialFunc    func(network, address string) (net.Conn, error)
}

// Entry describes a file and is returned by List().
type Entry struct {
	Name string
	Type EntryType
	Size uint64
	Time time.Time
}

// Dial connects to the specified address with optinal options
func Dial(addr string, options ...DialOption) (*ServerConn, error) {
	do := &dialOptions{}
	for _, option := range options {
		option.setup(do)
	}

	if do.location == nil {
		do.location = time.UTC
	}

	tconn := do.conn
	if tconn == nil {
		var err error

		if do.dialFunc != nil {
			tconn, err = do.dialFunc("tcp", addr)
		} else {
			ctx := do.context

			if ctx == nil {
				ctx = context.Background()
			}

			tconn, err = do.dialer.DialContext(ctx, "tcp", addr)
		}

		if err != nil {
			return nil, err
		}
	}

	// Use the resolved IP address in case addr contains a domain name
	// If we use the domain name, we might not resolve to the same IP.
	remoteAddr := tconn.RemoteAddr().(*net.TCPAddr)

	var sourceConn io.ReadWriteCloser = tconn
	if do.debugOutput != nil {
		sourceConn = newDebugWrapper(tconn, do.debugOutput)
	}

	c := &ServerConn{
		options:  do,
		features: make(map[string]string),
		conn:     textproto.NewConn(sourceConn),
		host:     remoteAddr.IP.String(),
		logger:   &logger.StdLogger{},
	}

	_, _, err := c.conn.ReadResponse(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	err = c.feat()
	if err != nil {
		c.Quit()
		return nil, err
	}

	if _, mlstSupported := c.features["MLST"]; mlstSupported {
		c.mlstSupported = true
	}

	return c, nil
}

// DialWithTimeout returns a DialOption that configures the ServerConn with specified timeout
func DialWithTimeout(timeout time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialer.Timeout = timeout
	}}
}

// DialWithDialer returns a DialOption that configures the ServerConn with specified net.Dialer
func DialWithDialer(dialer net.Dialer) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialer = dialer
	}}
}

// DialWithNetConn returns a DialOption that configures the ServerConn with the underlying net.Conn
func DialWithNetConn(conn net.Conn) DialOption {
	return DialOption{func(do *dialOptions) {
		do.conn = conn
	}}
}

// DialWithDisabledEPSV returns a DialOption that configures the ServerConn with EPSV disabled
// Note that EPSV is only used when advertised in the server features.
func DialWithDisabledEPSV(disabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.disableEPSV = disabled
	}}
}

// DialWithLocation returns a DialOption that configures the ServerConn with specified time.Location
// The lococation is used to parse the dates sent by the server which are in server's timezone
func DialWithLocation(location *time.Location) DialOption {
	return DialOption{func(do *dialOptions) {
		do.location = location
	}}
}

// DialWithContext returns a DialOption that configures the ServerConn with specified context
// The context will be used for the initial connection setup
func DialWithContext(ctx context.Context) DialOption {
	return DialOption{func(do *dialOptions) {
		do.context = ctx
	}}
}

// DialWithDebugOutput returns a DialOption that configures the ServerConn to write to the Writer
// everything it reads from the server
func DialWithDebugOutput(w io.Writer) DialOption {
	return DialOption{func(do *dialOptions) {
		do.debugOutput = w
	}}
}

// DialWithDialFunc returns a DialOption that configures the ServerConn to use the
// specified function to establish both control and data connections
//
// If used together with the DialWithNetConn option, the DialWithNetConn
// takes precedence for the control connection, while data connections will
// be established using function specified with the DialWithDialFunc option
func DialWithDialFunc(f func(network, address string) (net.Conn, error)) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialFunc = f
	}}
}

// Connect is an alias to Dial, for backward compatibility
func Connect(addr string) (*ServerConn, error) {
	return Dial(addr)
}

// DialTimeout initializes the connection to the specified ftp server address.
//
// It is generally followed by a call to Login() as most FTP commands require
// an authenticated user.
func DialTimeout(addr string, timeout time.Duration) (*ServerConn, error) {
	return Dial(addr, DialWithTimeout(timeout))
}
