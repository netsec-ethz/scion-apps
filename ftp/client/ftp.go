// Package ftp implements a FTP scionftp as described in RFC 959.
//
// A textproto.Error is returned for errors at the protocol level.
package ftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/scion"

	"github.com/netsec-ethz/scion-apps/ftp/logger"
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
	options               *dialOptions
	conn                  *textproto.Conn
	keepAliveConn         *textproto.Conn
	local, remote         string // local and remote address (without port!)
	localAddr, remoteAddr scion.Address
	features              map[string]string // Server capabilities discovered at runtime
	mlstSupported         bool
	retrHerculesSupported bool
	storHerculesSupported bool
	extended              bool
	blockSize             int
	logger                logger.Logger
}

// DialOption represents an option to start a new connection with Dial
type DialOption struct {
	setup func(do *dialOptions)
}

// dialOptions contains all the options set by DialOption.setup
type dialOptions struct {
	context     context.Context
	dialer      net.Dialer
	disableEPSV bool
	location    *time.Location
	debugOutput io.Writer
	blockSize   int
}

// Entry describes a file and is returned by List().
type Entry struct {
	Name string
	Type EntryType
	Size uint64
	Time time.Time
}

// Dial connects to the specified address with optional options
func Dial(local, remote string, options ...DialOption) (*ServerConn, error) {
	do := &dialOptions{}
	for _, option := range options {
		option.setup(do)
	}

	if do.location == nil {
		do.location = time.UTC
	}

	maxChunkSize := do.blockSize
	if maxChunkSize == 0 {
		maxChunkSize = 500
	}

	conn, kConn, err := scion.DialAddr(local, remote, true)
	if err != nil {
		return nil, err
	}

	var sourceConn io.ReadWriteCloser = conn
	if do.debugOutput != nil {
		sourceConn = newDebugWrapper(conn, do.debugOutput)
	}

	var sourceKConn io.ReadWriteCloser = kConn
	if do.debugOutput != nil {
		sourceKConn = newDebugWrapper(kConn, do.debugOutput)
	}

	localHost, _, err := scion.ParseAddress(local)
	if err != nil {
		return nil, err
	}

	remoteHost, _, err := scion.ParseAddress(remote)
	if err != nil {
		return nil, err
	}

	c := &ServerConn{
		options:       do,
		features:      make(map[string]string),
		conn:          textproto.NewConn(sourceConn),
		keepAliveConn: textproto.NewConn(sourceKConn),
		local:         localHost,
		remote:        remoteHost,
		localAddr:     conn.LocalAddress(),
		remoteAddr:    conn.RemoteAddress(),
		logger:        &logger.StdLogger{},
		blockSize:     maxChunkSize,
	}

	_, _, err = c.conn.ReadResponse(StatusReady)
	if err != nil {
		if err2 := c.Quit(); err2 != nil {
			return nil, fmt.Errorf("could not read response: %s\nand could not close connection: %s", err, err2)
		}
		return nil, fmt.Errorf("could not read response: %s", err)
	}

	err = c.feat()
	if err != nil {
		if err2 := c.Quit(); err2 != nil {
			return nil, fmt.Errorf("could execute FEAT: %s\nand could not close connection: %s", err, err2)
		}
		return nil, fmt.Errorf("could execute FEAT: %s", err)
	}

	if _, mlstSupported := c.features["MLST"]; mlstSupported {
		c.mlstSupported = true
	}
	if _, retrHerculesSupported := c.features["RETR_HERCULES"]; retrHerculesSupported {
		c.retrHerculesSupported = true
	}
	if _, storHerculesSupported := c.features["STOR_HERCULES"]; storHerculesSupported {
		c.storHerculesSupported = true
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

// DialWithDisabledEPSV returns a DialOption that configures the ServerConn with EPSV disabled
// Note that EPSV is only used when advertised in the server features.
func DialWithDisabledEPSV(disabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.disableEPSV = disabled
	}}
}

// DialWithLocation returns a DialOption that configures the ServerConn with specified time.Location
// The location is used to parse the dates sent by the server which are in server's timezone
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

// DialWithBlockSize sets the maximum blocksize to be used at the start but only clientside,
// alternatively we can set it with the command OPTS RETR (SetRetrOpts)
func DialWithBlockSize(blockSize int) DialOption {
	return DialOption{func(do *dialOptions) {
		do.blockSize = blockSize
	}}
}

// DialTimeout initializes the connection to the specified ftp server address.
//
// It is generally followed by a call to Login() as most FTP commands require
// an authenticated user.
func DialTimeout(local, remote string, timeout time.Duration) (*ServerConn, error) {
	return Dial(local, remote, DialWithTimeout(timeout))
}
