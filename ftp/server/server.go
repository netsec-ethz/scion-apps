// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"net"
	"strconv"

	"github.com/netsec-ethz/scion-apps/ftp/scion"

	"github.com/netsec-ethz/scion-apps/ftp/logger"
)

// Version returns the library version
func Version() string {
	return "0.3.1"
}

// ServerOpts contains parameters for server.NewServer()
type Opts struct {
	// The factory that will be used to create a new FTPDriver instance for
	// each scionftp connection. This is a mandatory option.
	Factory DriverFactory

	Auth Auth

	// Server Name, Default is Go Ftp Server
	Name string

	// The hostname that the FTP server should listen on. Optional, defaults to
	// "::", which means all hostnames on ipv4 and ipv6.
	Hostname string

	// The port that the FTP should listen on. Optional, defaults to 3000. In
	// a production environment you will probably want to change this to 21.
	Port uint16

	WelcomeMessage string

	// A logger implementation, if nil the StdLogger is used
	Logger logger.Logger

	Certificate *tls.Certificate

	// Hercules binary for RETR_HERCULES feature
	HerculesBinary string
	RootPath       string
}

// Server is the root of your FTP application. You should instantiate one
// of these and call ListenAndServe() to start accepting cilent connections.
//
// Always use the NewServer() method to create a new Server.
type Server struct {
	*Opts
	listenTo     string
	logger       logger.Logger
	listener     *scion.Listener
	ctx          context.Context
	cancel       context.CancelFunc
	feats        string
	herculesLock lock
}

// ErrServerClosed is returned by ListenAndServe() or Serve() when a shutdown
// was requested.
var ErrServerClosed = errors.New("ftp: Server closed")

// serverOptsWithDefaults copies an ServerOpts struct into a new struct,
// then adds any default values that are missing and returns the new data.
func serverOptsWithDefaults(opts *Opts) *Opts {
	var newOpts Opts
	if opts == nil {
		opts = &Opts{}
	}
	if opts.Hostname == "" {
		newOpts.Hostname = "::"
	} else {
		newOpts.Hostname = opts.Hostname
	}
	if opts.Port == 0 {
		newOpts.Port = 2121
	} else {
		newOpts.Port = opts.Port
	}
	newOpts.Factory = opts.Factory
	if opts.Name == "" {
		newOpts.Name = "Go FTP Server"
	} else {
		newOpts.Name = opts.Name
	}

	if opts.WelcomeMessage == "" {
		newOpts.WelcomeMessage = defaultWelcomeMessage
	} else {
		newOpts.WelcomeMessage = opts.WelcomeMessage
	}

	if opts.Auth != nil {
		newOpts.Auth = opts.Auth
	}

	newOpts.Logger = &logger.StdLogger{}
	if opts.Logger != nil {
		newOpts.Logger = opts.Logger
	}

	newOpts.Certificate = opts.Certificate
	newOpts.HerculesBinary = opts.HerculesBinary
	newOpts.RootPath = opts.RootPath

	return &newOpts
}

// NewServer initialises a new FTP server. Configuration options are provided
// via an instance of ServerOpts. Calling this function in your code will
// probably look something like this:
//
//     factory := &MyDriverFactory{}
//     server  := server.NewServer(&server.ServerOpts{ Factory: factory })
//
// or:
//
//     factory := &MyDriverFactory{}
//     opts    := &server.ServerOpts{
//       Factory: factory,
//       Port: 2000,
//       Hostname: "127.0.0.1",
//     }
//     server  := server.NewServer(opts)
//
func NewServer(opts *Opts) *Server {
	opts = serverOptsWithDefaults(opts)
	s := new(Server)
	s.Opts = opts
	s.listenTo = opts.Hostname + ":" + strconv.Itoa(int(opts.Port))
	s.logger = opts.Logger
	s.herculesLock = makeLock()
	return s
}

// NewConn constructs a new object that will handle the FTP protocol over
// an active net.TCPConn. The TCP connection should already be open before
// it is handed to this functions. driver is an instance of FTPDriver that
// will handle all auth and persistence details.
func (server *Server) newConn(tcpConn net.Conn, driver Driver) *Conn {
	c := new(Conn)
	c.namePrefix = "/"
	c.conn = tcpConn
	c.controlReader = bufio.NewReader(tcpConn)
	c.controlWriter = bufio.NewWriter(tcpConn)
	c.driver = driver
	c.auth = server.Auth
	c.server = server
	c.sessionID = newSessionID()
	c.logger = server.logger

	driver.Init(c)
	return c
}

// ListenAndServe asks a new Server to begin accepting scionftp connections. It
// accepts no arguments - all configuration is provided via the NewServer
// function.
//
// If the server fails to start for any reason, an error will be returned. Common
// errors are trying to bind to a privileged port or something else is already
// listening on the same port.
//
func (server *Server) ListenAndServe() error {
	var listener *scion.Listener
	var err error
	var curFeats = featCmds

	listener, err = scion.Listen(server.listenTo, server.Certificate)
	if err != nil {
		return err
	}

	if server.HerculesBinary != "" {
		curFeats += " RETR_HERCULES\n"
	}
	server.feats = fmt.Sprintf(feats, curFeats)

	sessionID := ""
	server.logger.Printf(sessionID, "%s listening on %d", server.Name, server.Port)

	return server.Serve(listener)
}

// Serve accepts connections on a given net.Listener and handles each
// request in a new goroutine.
//
func (server *Server) Serve(l *scion.Listener) error {
	server.listener = l
	server.ctx, server.cancel = context.WithCancel(context.Background())
	sessionID := ""
	for {
		conn, session, err := server.listener.Accept()
		if err != nil {
			select {
			case <-server.ctx.Done():
				return ErrServerClosed
			default:
			}
			server.logger.Printf(sessionID, "listening error: %v", err)
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}
		driver, err := server.Factory.NewDriver()
		if err != nil {
			server.logger.Printf(sessionID, "Error creating driver, aborting scionftp connection: %v", err)
			conn.Close()
		} else {
			ftpConn := server.newConn(conn, driver)
			go ftpConn.Serve()
			go acceptKeepAlive(session, ftpConn)
		}
	}
}

func acceptKeepAlive(session *quic.Session, ftpConn *Conn) {
	stream, err := scion.AcceptStream(session)
	if err != nil {
		if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
			ftpConn.logger.Printf(ftpConn.sessionID, "could not accept keep alive stream: %s", err)
		}
		return
	}
	ftpConn.AddKeepAliveConn(&stream)
	ftpConn.ServeKeepAlive()
}

// Shutdown will gracefully stop a server. Already connected clients will retain their connections
func (server *Server) Shutdown() error {
	if server.cancel != nil {
		server.cancel()
	}
	if server.listener != nil {
		return server.listener.Close()
	}
	// server wasn't even started
	return nil
}
