// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"bufio"
	crypto "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netsec-ethz/scion-apps/ftp/scion"

	"github.com/netsec-ethz/scion-apps/ftp/logger"
	"github.com/netsec-ethz/scion-apps/ftp/socket"
)

const (
	defaultWelcomeMessage = "Welcome to the Go FTP Server"
	listenerRetries       = 10
)

type Conn struct {
	conn            net.Conn
	controlReader   *bufio.Reader
	controlWriter   *bufio.Writer
	keepAliveConn   net.Conn
	keepAliveReader *bufio.Reader
	keepAliveWriter *bufio.Writer
	dataConn        socket.DataSocket
	driver          Driver
	auth            Auth
	herculesPort    uint16
	logger          logger.Logger
	server          *Server
	sessionID       string
	namePrefix      string
	reqUser         string
	user            string
	renameFrom      string
	lastFilePos     int64
	appendData      bool
	closed          bool
	extended        bool
	parallelism     int
	blockSize       int
}

func (conn *Conn) LoginUser() string {
	return conn.user
}

func (conn *Conn) IsLogin() bool {
	return len(conn.user) > 0
}

func (conn *Conn) NewListener() (*scion.Listener, error) {

	var err error
	var listener *scion.Listener

	for i := 0; i < listenerRetries; i++ {

		listener, err = scion.Listen(conn.server.Hostname+":0", conn.server.Certificate)
		if err == nil {
			break
		}
	}

	return listener, err
}

// returns a random 20 char string that can be used as a unique session ID
func newSessionID() string {
	hash := sha256.New()
	_, err := io.CopyN(hash, crypto.Reader, 50)
	if err != nil {
		return "????????????????????"
	}
	md := hash.Sum(nil)
	mdStr := hex.EncodeToString(md)
	return mdStr[0:20]
}

// Serve starts an endless loop that reads FTP commands from the scionftp and
// responds appropriately. terminated is a channel that will receive a true
// message when the connection closes. This loop will be running inside a
// goroutine, so use this channel to be notified when the connection can be
// cleaned up.
func (conn *Conn) Serve() {
	conn.logger.Print(conn.sessionID, "Connection Established")
	// send welcome
	_, err := conn.writeMessage(220, conn.server.WelcomeMessage)
	if err != nil {
		conn.logger.Print(conn.sessionID, fmt.Sprint("write error:", err))
	}
	// read commands
	for {
		line, err := conn.controlReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				conn.logger.Print(conn.sessionID, fmt.Sprint("read error:", err))
			}

			break
		}
		conn.receiveLine(line)
		// QUIT command closes connection, break to avoid error on reading from
		// closed socket
		if conn.closed {
			break
		}
	}
	conn.Close()
	conn.logger.Print(conn.sessionID, "Connection Terminated")
}

func (conn *Conn) AddKeepAliveConn(stream *quic.Stream) {
	keepAliveConn := scion.NewAppQuicConnection(*stream, conn.conn.LocalAddr().(scion.Address), conn.conn.RemoteAddr().(scion.Address))
	conn.keepAliveConn = keepAliveConn
	conn.keepAliveReader = bufio.NewReader(keepAliveConn)
	conn.keepAliveWriter = bufio.NewWriter(keepAliveConn)
}

// ServeKeepAlive starts an endless loop that only accepts and responds to NOOP
// commands. A scionftp client can send keep alive packets using the separate
// stream to avoid race conditions on the primary stream during data transfers.
func (conn *Conn) ServeKeepAlive() {
	conn.logger.Print(conn.sessionID, "Keep-Alive Stream Established")
	// read commands
	for {
		line, err := conn.keepAliveReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				conn.logger.Print(conn.sessionID, fmt.Sprint("read error:", err))
			}

			break
		}
		command := strings.ToUpper(strings.Trim(line, "\r\n"))
		if command != "NOOP" {
			_, _ = conn.writeMessage(550, "Got non-NOOP on keep-alive stream")
		} else {
			_, _ = conn.keepAliveWriter.WriteString("200 OK\r\n")
			_ = conn.keepAliveWriter.Flush()
		}
		// QUIT command closes connection, break to avoid error on reading from
		// closed socket
		if conn.closed {
			break
		}
	}
	conn.Close()
	conn.logger.Print(conn.sessionID, "Keep-Alive Stream Terminated")
}

// Close will manually close this connection, even if the scionftp isn't ready.
func (conn *Conn) Close() {
	_ = conn.conn.Close()
	if conn.keepAliveConn != nil {
		_ = conn.keepAliveConn.Close()
	}
	conn.closed = true
	if conn.dataConn != nil {
		_ = conn.dataConn.Close()
		conn.dataConn = nil
	}
}

// receiveLine accepts a single line FTP command and co-ordinates an
// appropriate response.
func (conn *Conn) receiveLine(line string) {
	command, param := conn.parseLine(line)
	conn.logger.PrintCommand(conn.sessionID, command, param)
	cmdObj := commands[strings.ToUpper(command)]
	if cmdObj == nil {
		_, _ = conn.writeMessage(500, "Command not found")
		return
	}
	if cmdObj.RequireParam() && param == "" {
		_, _ = conn.writeMessage(553, "action aborted, required param missing")
	} else if cmdObj.RequireAuth() && conn.user == "" {
		_, _ = conn.writeMessage(530, "not logged in")
	} else {
		cmdObj.Execute(conn, param)
	}
}

func (conn *Conn) parseLine(line string) (string, string) {
	params := strings.SplitN(strings.Trim(line, "\r\n"), " ", 2)
	if len(params) == 1 {
		return params[0], ""
	}
	return params[0], strings.TrimSpace(params[1])
}

// writeMessage will send a standard FTP response back to the scionftp.
func (conn *Conn) writeMessage(code int, message string) (wrote int, err error) {
	conn.logger.PrintResponse(conn.sessionID, code, message)
	line := fmt.Sprintf("%d %s\r\n", code, message)
	wrote, err = conn.controlWriter.WriteString(line)
	if err == nil {
		err = conn.controlWriter.Flush()
	}
	return
}

func (conn *Conn) writeOrLog(code int, message string) {
	_, err := conn.writeMessage(code, message)
	if err != nil {
		log.Printf("Could not write message (%d %s): %s", code, message, err)
	}
}

// writeMessage will send a standard FTP response back to the scionftp.
func (conn *Conn) writeMessageMultiline(code int, message string) (wrote int, err error) {
	conn.logger.PrintResponse(conn.sessionID, code, message)
	line := fmt.Sprintf("%d-%s\r\n%d END\r\n", code, message, code)
	wrote, err = conn.controlWriter.WriteString(line)
	if err == nil {
		err = conn.controlWriter.Flush()
	}
	return
}

// buildPath takes a scionftp supplied path or filename and generates a safe
// absolute path within their account sandbox.
//
//    buildpath("/")
//    => "/"
//    buildpath("one.txt")
//    => "/one.txt"
//    buildpath("/files/two.txt")
//    => "/files/two.txt"
//    buildpath("files/two.txt")
//    => "/files/two.txt"
//    buildpath("/../../../../etc/passwd")
//    => "/etc/passwd"
//
// The driver implementation is responsible for deciding how to treat this path.
// Obviously they MUST NOT just read the path off disk. The probably want to
// prefix the path with something to scope the users access to a sandbox.
func (conn *Conn) buildPath(filename string) (fullPath string) {
	if len(filename) > 0 && filename[0:1] == "/" {
		fullPath = filepath.Clean(filename)
	} else if len(filename) > 0 && filename != "-a" {
		fullPath = filepath.Clean(conn.namePrefix + "/" + filename)
	} else {
		fullPath = filepath.Clean(conn.namePrefix)
	}
	fullPath = strings.Replace(fullPath, "//", "/", -1)
	fullPath = strings.Replace(fullPath, string(filepath.Separator), "/", -1)
	return
}

// sendOutofbandData will send a string to the scionftp via the currently open
// data socket. Assumes the socket is open and ready to be used.
func (conn *Conn) sendOutofbandData(data []byte) {
	bytes := len(data)
	if conn.dataConn != nil {
		_, _ = conn.dataConn.Write(data)
		_ = conn.dataConn.Close()
		conn.dataConn = nil
	}
	message := "Closing data connection, sent " + strconv.Itoa(bytes) + " bytes"
	_, _ = conn.writeMessage(226, message)
}

func (conn *Conn) sendOutofBandDataWriter(data io.ReadCloser) error {
	conn.lastFilePos = 0
	bytes, err := io.Copy(conn.dataConn, data)
	if err != nil {
		err = conn.dataConn.Close()
		conn.dataConn = nil
		return err
	}
	message := "Closing data connection, sent " + strconv.Itoa(int(bytes)) + " bytes"
	_, err = conn.writeMessage(226, message)
	if err == nil {
		err = conn.dataConn.Close()
	}
	conn.dataConn = nil

	return err
}
