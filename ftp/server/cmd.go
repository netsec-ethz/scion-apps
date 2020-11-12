// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"github.com/netsec-ethz/scion-apps/ftp/mode"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/scion"

	socket2 "github.com/netsec-ethz/scion-apps/ftp/socket"
)

type Command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(*Conn, string)
}

type commandMap map[string]Command

var (
	commands = commandMap{
		"ADAT":          commandAdat{},
		"ALLO":          commandAllo{},
		"APPE":          commandAppe{},
		"CDUP":          commandCdup{},
		"CWD":           commandCwd{},
		"CCC":           commandCcc{},
		"CONF":          commandConf{},
		"DELE":          commandDele{},
		"ENC":           commandEnc{},
		"EPRT":          commandEprt{},
		"EPSV":          commandEpsv{},
		"FEAT":          commandFeat{},
		"HERCULES_PORT": commandHerculesPort{},
		"LIST":          commandList{},
		"LPRT":          commandLprt{},
		"NLST":          commandNlst{},
		"MDTM":          commandMdtm{},
		"MIC":           commandMic{},
		"MKD":           commandMkd{},
		"MODE":          commandMode{},
		"NOOP":          commandNoop{},
		"OPTS":          commandOpts{},
		"PASS":          commandPass{},
		"PASV":          commandPasv{},
		"PBSZ":          commandPbsz{},
		"PORT":          commandPort{},
		"PROT":          commandProt{},
		"PWD":           commandPwd{},
		"QUIT":          commandQuit{},
		"RETR":          commandRetr{},
		"RETR_HERCULES": commandRetrHercules{},
		"REST":          commandRest{},
		"RNFR":          commandRnfr{},
		"RNTO":          commandRnto{},
		"RMD":           commandRmd{},
		"SIZE":          commandSize{},
		"STOR":          commandStor{},
		"STOR_HERCULES": commandStorHercules{},
		"STRU":          commandStru{},
		"SYST":          commandSyst{},
		"TYPE":          commandType{},
		"USER":          commandUser{},
		"XCUP":          commandCdup{},
		"XCWD":          commandCwd{},
		"XMKD":          commandMkd{},
		"XPWD":          commandPwd{},
		"XRMD":          commandRmd{},
		"SPAS":          commandSpas{},
		"ERET":          commandEret{},
	}
)

// commandAllo responds to the ALLO FTP command.
//
// This is essentially a ping from the scionftp so we just respond with an
// basic OK message.
type commandAllo struct{}

func (cmd commandAllo) IsExtend() bool {
	return false
}

func (cmd commandAllo) RequireParam() bool {
	return false
}

func (cmd commandAllo) RequireAuth() bool {
	return false
}

func (cmd commandAllo) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(202, "Obsolete")
}

// commandAppe responds to the APPE FTP command. It allows the user to upload a
// new file but always append if file exists otherwise create one.
type commandAppe struct{}

func (cmd commandAppe) IsExtend() bool {
	return false
}

func (cmd commandAppe) RequireParam() bool {
	return true
}

func (cmd commandAppe) RequireAuth() bool {
	return true
}

func (cmd commandAppe) Execute(conn *Conn, param string) {
	targetPath := conn.buildPath(param)
	_, _ = conn.writeMessage(150, "Data transfer starting")

	bytes, err := conn.driver.PutFile(targetPath, conn.dataConn, true)
	if err == nil {
		msg := "OK, received " + strconv.Itoa(int(bytes)) + " bytes"
		_, _ = conn.writeMessage(226, msg)
	} else {
		_, _ = conn.writeMessage(450, fmt.Sprint("error during transfer: ", err))
	}
}

type commandOpts struct{}

func (cmd commandOpts) IsExtend() bool {
	return false
}

func (cmd commandOpts) RequireParam() bool {
	return false
}

func (cmd commandOpts) RequireAuth() bool {
	return false
}
func (cmd commandOpts) Execute(conn *Conn, param string) {
	parts := strings.Fields(param)
	if len(parts) != 2 {
		_, _ = conn.writeMessage(550, "Unknown params")
		return
	}

	switch strings.ToUpper(parts[0]) {
	case "UTF8":
		if strings.ToUpper(parts[1]) == "ON" {
			_, _ = conn.writeMessage(200, "UTF8 mode enabled")
		} else {
			_, _ = conn.writeMessage(550, "Unsupported non-utf8 mode")
		}
	case "RETR":
		parallelism, blockSize, err := ParseOptions(parts[1])
		if err != nil {
			_, _ = conn.writeMessage(550, fmt.Sprintf("failed to parse options: %s", err))
		} else {
			conn.parallelism = parallelism
			conn.blockSize = blockSize
			_, _ = conn.writeMessage(200, fmt.Sprintf("Parallelism set to %d", parallelism))
		}
	default:
		_, _ = conn.writeMessage(550, "Unknown params")
	}

}

func ParseOptions(param string) (parallelism, blockSize int, err error) {
	parts := strings.Split(strings.TrimRight(param, ";"), ";")
	for _, part := range parts {
		splitted := strings.Split(part, "=")
		if len(splitted) != 2 {
			err = fmt.Errorf("unknown params")
			return
		}

		switch strings.ToUpper(splitted[0]) {
		case "PARALLELISM":
			parallelism, err = strconv.Atoi(splitted[1])
			if err != nil || parallelism < 1 {
				err = fmt.Errorf("unknown params")
				return
			}
		case "STRIPELAYOUT":
			if strings.ToUpper(splitted[1]) != "BLOCKED" {
				err = fmt.Errorf("only blocked mode supported")
				return
			}
		case "BLOCKSIZE":
			blockSize, err = strconv.Atoi(splitted[1])
			if err != nil || blockSize < 1 {
				err = fmt.Errorf("unknown params")
				return
			}
		}
	}

	return
}

type commandFeat struct{}

func (cmd commandFeat) IsExtend() bool {
	return false
}

func (cmd commandFeat) RequireParam() bool {
	return false
}

func (cmd commandFeat) RequireAuth() bool {
	return false
}

var (
	feats    = "Extensions supported:\n%s"
	featCmds = " UTF8\n"
)

func init() {
	for k, v := range commands {
		if v.IsExtend() {
			featCmds = featCmds + " " + k + "\n"
		}
	}
}

func (cmd commandFeat) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessageMultiline(211, conn.server.feats)
}

// cmdCdup responds to the CDUP FTP command.
//
// Allows the scionftp change their current directory to the parent.
type commandCdup struct{}

func (cmd commandCdup) IsExtend() bool {
	return false
}

func (cmd commandCdup) RequireParam() bool {
	return false
}

func (cmd commandCdup) RequireAuth() bool {
	return true
}

func (cmd commandCdup) Execute(conn *Conn, param string) {
	otherCmd := &commandCwd{}
	otherCmd.Execute(conn, "..")
}

// commandCwd responds to the CWD FTP command. It allows the scionftp to change the
// current working directory.
type commandCwd struct{}

func (cmd commandCwd) IsExtend() bool {
	return false
}

func (cmd commandCwd) RequireParam() bool {
	return true
}

func (cmd commandCwd) RequireAuth() bool {
	return true
}

func (cmd commandCwd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	err := conn.driver.ChangeDir(path)
	if err == nil {
		conn.namePrefix = path
		_, _ = conn.writeMessage(250, "Directory changed to "+path)
	} else {
		_, _ = conn.writeMessage(550, fmt.Sprint("Directory change to ", path, " failed: ", err))
	}
}

// commandDele responds to the DELE FTP command. It allows the scionftp to delete
// a file
type commandDele struct{}

func (cmd commandDele) IsExtend() bool {
	return false
}

func (cmd commandDele) RequireParam() bool {
	return true
}

func (cmd commandDele) RequireAuth() bool {
	return true
}

func (cmd commandDele) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	err := conn.driver.DeleteFile(path)
	if err == nil {
		_, _ = conn.writeMessage(250, "File deleted")
	} else {
		_, _ = conn.writeMessage(550, fmt.Sprint("File delete failed: ", err))
	}
}

// commandEprt responds to the EPRT FTP command. It allows the scionftp to
// request an active data socket with more options than the original PORT
// command. It mainly adds ipv6 support.
type commandEprt struct{}

func (cmd commandEprt) IsExtend() bool {
	return true
}

func (cmd commandEprt) RequireParam() bool {
	return true
}

func (cmd commandEprt) RequireAuth() bool {
	return true
}

func (cmd commandEprt) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(502, "Active mode not supported, use passive mode instead")
}

// commandLprt responds to the LPRT FTP command. It allows the scionftp to
// request an active data socket with more options than the original PORT
// command.  FTP Operation Over Big Address Records.
type commandLprt struct{}

func (cmd commandLprt) IsExtend() bool {
	return true
}

func (cmd commandLprt) RequireParam() bool {
	return true
}

func (cmd commandLprt) RequireAuth() bool {
	return true
}

func (cmd commandLprt) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(502, "Active mode not supported, use passive mode instead")

}

// commandEpsv responds to the EPSV FTP command. It allows the speedtest_client to
// request a passive data socket with more options than the original PASV
// command. It mainly adds ipv6 support, although we don't support that yet.
type commandEpsv struct{}

func (cmd commandEpsv) IsExtend() bool {
	return true
}

func (cmd commandEpsv) RequireParam() bool {
	return false
}

func (cmd commandEpsv) RequireAuth() bool {
	return true
}

func (cmd commandEpsv) Execute(conn *Conn, param string) {

	listener, err := conn.NewListener()

	if err != nil {
		log.Println(err)
		_, _ = conn.writeMessage(425, "Data connection failed")
		return
	}
	msg := fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", listener.Port())
	_, _ = conn.writeMessage(229, msg)

	socket, _, err := listener.Accept()
	if err != nil {
		_, _ = conn.writeMessage(426, "Connection closed, failed to open data connection")
		return
	}
	conn.dataConn = socket2.NewScionSocket(socket)

}

// commandHerculesPort responds to the ALLO FTP command.
//
// This is essentially a ping from the scionftp so we just respond with an
// basic OK message.
type commandHerculesPort struct{}

func (cmd commandHerculesPort) IsExtend() bool {
	return false
}

func (cmd commandHerculesPort) RequireParam() bool {
	return true
}

func (cmd commandHerculesPort) RequireAuth() bool {
	return true
}

func (cmd commandHerculesPort) Execute(conn *Conn, param string) {
	port, err := strconv.ParseUint(param, 10, 16)
	if err != nil {
		_, _ = conn.writeMessage(529, "Invalid port number")
	}
	conn.herculesPort = uint16(port)
	_, _ = conn.writeMessage(320, "Ok")
}

// commandList responds to the LIST FTP command. It allows the speedtest_client to retrieve
// a detailed listing of the contents of a directory.
type commandList struct{}

func (cmd commandList) IsExtend() bool {
	return false
}

func (cmd commandList) RequireParam() bool {
	return false
}

func (cmd commandList) RequireAuth() bool {
	return true
}

func (cmd commandList) Execute(conn *Conn, param string) {

	fmt.Println("List!")

	path := conn.buildPath(parseListParam(param))
	info, err := conn.driver.Stat(path)
	if err != nil {
		_, _ = conn.writeMessage(550, err.Error())
		return
	}

	if info == nil {
		conn.logger.Printf(conn.sessionID, "%s: no such file or directory.\n", path)
		return
	}
	var files []FileInfo
	if info.IsDir() {
		err = conn.driver.ListDir(path, func(f FileInfo) error {
			files = append(files, f)
			return nil
		})
		if err != nil {
			_, _ = conn.writeMessage(550, err.Error())
			return
		}
	} else {
		files = append(files, info)
	}

	_, _ = conn.writeMessage(150, "Opening ASCII mode data connection for file list")
	conn.sendOutofbandData(listFormatter(files).Detailed())
}

func parseListParam(param string) (path string) {
	if len(param) == 0 {
		path = param
	} else {
		fields := strings.Fields(param)
		i := 0
		for _, field := range fields {
			if !strings.HasPrefix(field, "-") {
				break
			}
			i = strings.LastIndex(param, " "+field) + len(field) + 1
		}
		path = strings.TrimLeft(param[i:], " ") //Get all the path even with space inside
	}
	return path
}

// commandNlst responds to the NLST FTP command. It allows the speedtest_client to
// retrieve a list of filenames in the current directory.
type commandNlst struct{}

func (cmd commandNlst) IsExtend() bool {
	return false
}

func (cmd commandNlst) RequireParam() bool {
	return false
}

func (cmd commandNlst) RequireAuth() bool {
	return true
}

func (cmd commandNlst) Execute(conn *Conn, param string) {
	path := conn.buildPath(parseListParam(param))
	info, err := conn.driver.Stat(path)
	if err != nil {
		_, _ = conn.writeMessage(550, err.Error())
		return
	}
	if !info.IsDir() {
		_, _ = conn.writeMessage(550, param+" is not a directory")
		return
	}

	var files []FileInfo
	err = conn.driver.ListDir(path, func(f FileInfo) error {
		files = append(files, f)
		return nil
	})
	if err != nil {
		_, _ = conn.writeMessage(550, err.Error())
		return
	}
	_, _ = conn.writeMessage(150, "Opening ASCII mode data connection for file list")
	conn.sendOutofbandData(listFormatter(files).Short())
}

// commandMdtm responds to the MDTM FTP command. It allows the speedtest_client to
// retrieve the last modified time of a file.
type commandMdtm struct{}

func (cmd commandMdtm) IsExtend() bool {
	return false
}

func (cmd commandMdtm) RequireParam() bool {
	return true
}

func (cmd commandMdtm) RequireAuth() bool {
	return true
}

func (cmd commandMdtm) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	stat, err := conn.driver.Stat(path)
	if err == nil {
		_, _ = conn.writeMessage(213, stat.ModTime().Format("20060102150405"))
	} else {
		_, _ = conn.writeMessage(450, "File not available")
	}
}

// commandMkd responds to the MKD FTP command. It allows the speedtest_client to create
// a new directory
type commandMkd struct{}

func (cmd commandMkd) IsExtend() bool {
	return false
}

func (cmd commandMkd) RequireParam() bool {
	return true
}

func (cmd commandMkd) RequireAuth() bool {
	return true
}

func (cmd commandMkd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	err := conn.driver.MakeDir(path)
	if err == nil {
		_, _ = conn.writeMessage(257, "Directory created")
	} else {
		_, _ = conn.writeMessage(550, fmt.Sprint("Action not taken: ", err))
	}
}

// cmdMode responds to the MODE FTP command.
//
// the original FTP spec had various options for hosts to negotiate how data
// would be sent over the data socket, In reality these days (S)tream mode
// is all that is used for the mode - data is just streamed down the data
// socket unchanged.
type commandMode struct{}

func (cmd commandMode) IsExtend() bool {
	return false
}

func (cmd commandMode) RequireParam() bool {
	return true
}

func (cmd commandMode) RequireAuth() bool {
	return true
}

func (cmd commandMode) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "S" {
		// Stream Mode
		conn.extended = false
		_, _ = conn.writeMessage(200, "OK")

	} else if strings.ToUpper(param) == "E" {
		// Extended Block Mode
		conn.extended = true
		conn.parallelism = 4
		conn.blockSize = 500
		_, _ = conn.writeMessage(200, "OK")

	} else {
		_, _ = conn.writeMessage(504, "MODE is an obsolete command, only (S)tream and (E)xtended Mode supported")
	}
}

// cmdNoop responds to the NOOP FTP command.
//
// This is essentially a ping from the speedtest_client so we just respond with an
// basic 200 message.
type commandNoop struct{}

func (cmd commandNoop) IsExtend() bool {
	return false
}

func (cmd commandNoop) RequireParam() bool {
	return false
}

func (cmd commandNoop) RequireAuth() bool {
	return false
}

func (cmd commandNoop) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(200, "OK")
}

// commandPass respond to the PASS FTP command by asking the driver if the
// supplied username and password are valid
type commandPass struct{}

func (cmd commandPass) IsExtend() bool {
	return false
}

func (cmd commandPass) RequireParam() bool {
	return true
}

func (cmd commandPass) RequireAuth() bool {
	return false
}

func (cmd commandPass) Execute(conn *Conn, param string) {
	ok, err := conn.server.Auth.CheckPasswd(conn.reqUser, param)
	if err != nil {
		_, _ = conn.writeMessage(550, "Checking password error")
		return
	}

	if ok {
		conn.user = conn.reqUser
		conn.reqUser = ""
		_, _ = conn.writeMessage(230, "Password ok, continue")
	} else {
		_, _ = conn.writeMessage(530, "Incorrect password, not logged in")
	}
}

// commandPasv responds to the PASV FTP command.
//
// The speedtest_client is requesting us to open a new TCP listing socket and wait for them
// to connect to it.
type commandPasv struct{}

func (cmd commandPasv) IsExtend() bool {
	return false
}

func (cmd commandPasv) RequireParam() bool {
	return false
}

func (cmd commandPasv) RequireAuth() bool {
	return true
}

func (cmd commandPasv) Execute(conn *Conn, param string) {
	commandEpsv{}.Execute(conn, param)
}

// commandPort responds to the PORT FTP command.
//
// The speedtest_client has opened a listening socket for sending out of band data and
// is requesting that we connect to it
type commandPort struct{}

func (cmd commandPort) IsExtend() bool {
	return false
}

func (cmd commandPort) RequireParam() bool {
	return true
}

func (cmd commandPort) RequireAuth() bool {
	return true
}

func (cmd commandPort) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(502, "Active mode not supported, use passive mode instead")
}

// commandPwd responds to the PWD FTP command.
//
// Tells the speedtest_client what the current working directory is.
type commandPwd struct{}

func (cmd commandPwd) IsExtend() bool {
	return false
}

func (cmd commandPwd) RequireParam() bool {
	return false
}

func (cmd commandPwd) RequireAuth() bool {
	return true
}

func (cmd commandPwd) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(257, "\""+conn.namePrefix+"\" is the current directory")
}

// CommandQuit responds to the QUIT FTP command. The speedtest_client has requested the
// connection be closed.
type commandQuit struct{}

func (cmd commandQuit) IsExtend() bool {
	return false
}

func (cmd commandQuit) RequireParam() bool {
	return false
}

func (cmd commandQuit) RequireAuth() bool {
	return false
}

func (cmd commandQuit) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(221, "Goodbye")
	conn.Close()
}

// commandRetr responds to the RETR FTP command. It allows the speedtest_client to
// download a file.
type commandRetr struct{}

func (cmd commandRetr) IsExtend() bool {
	return false
}

func (cmd commandRetr) RequireParam() bool {
	return true
}

func (cmd commandRetr) RequireAuth() bool {
	return true
}

func (cmd commandRetr) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	defer func() {
		conn.lastFilePos = 0
		conn.appendData = false
	}()
	bytes, data, err := conn.driver.GetFile(path, conn.lastFilePos)
	if err == nil {
		defer func() { _ = data.Close() }()
		_, _ = conn.writeMessage(150, fmt.Sprintf("Data transfer starting %v bytes", bytes))
		err = conn.sendOutofBandDataWriter(data)
		if err != nil {
			_, _ = conn.writeMessage(551, "Error reading file")
		}
	} else {
		_, _ = conn.writeMessage(551, "File not available")
	}
}

type commandRetrHercules struct{}

func (cmd commandRetrHercules) IsExtend() bool {
	return false
}

func (cmd commandRetrHercules) RequireParam() bool {
	return true
}

func (cmd commandRetrHercules) RequireAuth() bool {
	return true
}

func (cmd commandRetrHercules) Execute(conn *Conn, param string) {
	if conn.server.HerculesBinary == "" {
		conn.writeOrLog(502, "Command not implemented")
		return
	}

	// check file access as unprivileged user
	path := conn.server.RootPath + conn.buildPath(param)
	f, err := os.Open(path)
	if err != nil {
		conn.writeOrLog(551, "File not available for download")
		return
	}
	defer func() { _ = f.Close() }()

	log.Printf("No Hercules configuration given, using defaults (queue 0, copy mode)")

	if !conn.server.herculesLock.tryLockTimeout(5 * time.Second) {
		conn.writeOrLog(425, "All Hercules units busy - please try again later")
		return
	}
	defer conn.server.herculesLock.unlock()

	args := []string{
		conn.server.HerculesBinary,
		"-t", path,
	}

	port, err := scion.AllocateUDPPort(conn.conn.LocalAddr().String())
	if err != nil {
		log.Printf("could not allocate port: %s", err.Error())
		conn.writeOrLog(425, "Can't open data connection")
		return
	}
	localAddr := conn.conn.LocalAddr().(scion.Address).Addr()
	localAddr.Host.Port = int(port)
	remoteAddr := conn.conn.RemoteAddr().(scion.Address).Addr()
	remoteAddr.Host.Port = int(conn.herculesPort)
	args = append(args, "-l", localAddr.String())
	args = append(args, "-d", remoteAddr.String())

	iface, err := scion.FindInterfaceName(localAddr.Host.IP)
	if err != nil {
		log.Printf("could not find interface: %s", err)
		conn.writeOrLog(425, "Can't open data connection")
		return
	}
	args = append(args, "-i", iface)

	command := exec.Command("sudo", args...)
	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	_, err = conn.writeMessage(150, "Data transfer starting via Hercules")
	if err != nil {
		log.Printf("could not write response: %s", err.Error())
		return
	}

	log.Printf("run Hercules: %s", command)
	err = command.Run()
	if err != nil {
		// TODO improve error handling
		log.Printf("could not execute Hercules: %s", err)
		conn.writeOrLog(551, "Hercules returned an error")
	} else {
		conn.writeOrLog(226, "Hercules transfer complete")
	}
}

type commandRest struct{}

func (cmd commandRest) IsExtend() bool {
	return false
}

func (cmd commandRest) RequireParam() bool {
	return true
}

func (cmd commandRest) RequireAuth() bool {
	return true
}

func (cmd commandRest) Execute(conn *Conn, param string) {
	var err error
	conn.lastFilePos, err = strconv.ParseInt(param, 10, 64)
	if err != nil {
		_, _ = conn.writeMessage(551, "File not available")
		return
	}

	conn.appendData = true

	_, _ = conn.writeMessage(350, fmt.Sprint("Start transfer from ", conn.lastFilePos))
}

// commandRnfr responds to the RNFR FTP command. It's the first of two commands
// required for a speedtest_client to rename a file.
type commandRnfr struct{}

func (cmd commandRnfr) IsExtend() bool {
	return false
}

func (cmd commandRnfr) RequireParam() bool {
	return true
}

func (cmd commandRnfr) RequireAuth() bool {
	return true
}

func (cmd commandRnfr) Execute(conn *Conn, param string) {
	conn.renameFrom = conn.buildPath(param)
	_, _ = conn.writeMessage(350, "Requested file action pending further information.")
}

// cmdRnto responds to the RNTO FTP command. It's the second of two commands
// required for a speedtest_client to rename a file.
type commandRnto struct{}

func (cmd commandRnto) IsExtend() bool {
	return false
}

func (cmd commandRnto) RequireParam() bool {
	return true
}

func (cmd commandRnto) RequireAuth() bool {
	return true
}

func (cmd commandRnto) Execute(conn *Conn, param string) {
	toPath := conn.buildPath(param)
	err := conn.driver.Rename(conn.renameFrom, toPath)
	defer func() {
		conn.renameFrom = ""
	}()

	if err == nil {
		_, _ = conn.writeMessage(250, "File renamed")
	} else {
		_, _ = conn.writeMessage(550, fmt.Sprint("Action not taken: ", err))
	}
}

// cmdRmd responds to the RMD FTP command. It allows the speedtest_client to delete a
// directory.
type commandRmd struct{}

func (cmd commandRmd) IsExtend() bool {
	return false
}

func (cmd commandRmd) RequireParam() bool {
	return true
}

func (cmd commandRmd) RequireAuth() bool {
	return true
}

func (cmd commandRmd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	err := conn.driver.DeleteDir(path)
	if err == nil {
		_, _ = conn.writeMessage(250, "Directory deleted")
	} else {
		_, _ = conn.writeMessage(550, fmt.Sprint("Directory delete failed: ", err))
	}
}

type commandAdat struct{}

func (cmd commandAdat) IsExtend() bool {
	return false
}

func (cmd commandAdat) RequireParam() bool {
	return true
}

func (cmd commandAdat) RequireAuth() bool {
	return true
}

func (cmd commandAdat) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(550, "Action not taken")
}

type commandCcc struct{}

func (cmd commandCcc) IsExtend() bool {
	return false
}

func (cmd commandCcc) RequireParam() bool {
	return true
}

func (cmd commandCcc) RequireAuth() bool {
	return true
}

func (cmd commandCcc) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(550, "Action not taken")
}

type commandEnc struct{}

func (cmd commandEnc) IsExtend() bool {
	return false
}

func (cmd commandEnc) RequireParam() bool {
	return true
}

func (cmd commandEnc) RequireAuth() bool {
	return true
}

func (cmd commandEnc) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(550, "Action not taken")
}

type commandMic struct{}

func (cmd commandMic) IsExtend() bool {
	return false
}

func (cmd commandMic) RequireParam() bool {
	return true
}

func (cmd commandMic) RequireAuth() bool {
	return true
}

func (cmd commandMic) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(550, "Action not taken")
}

type commandPbsz struct{}

func (cmd commandPbsz) IsExtend() bool {
	return false
}

func (cmd commandPbsz) RequireParam() bool {
	return true
}

func (cmd commandPbsz) RequireAuth() bool {
	return false
}

func (cmd commandPbsz) Execute(conn *Conn, param string) {
	if param == "0" {
		_, _ = conn.writeMessage(200, "OK")
	} else {
		_, _ = conn.writeMessage(550, "Action not taken")
	}
}

type commandProt struct{}

func (cmd commandProt) IsExtend() bool {
	return false
}

func (cmd commandProt) RequireParam() bool {
	return true
}

func (cmd commandProt) RequireAuth() bool {
	return false
}

func (cmd commandProt) Execute(conn *Conn, param string) {
	if param == "P" {
		_, _ = conn.writeMessage(200, "OK")
	} else {
		_, _ = conn.writeMessage(550, "Action not taken")
	}
}

type commandConf struct{}

func (cmd commandConf) IsExtend() bool {
	return false
}

func (cmd commandConf) RequireParam() bool {
	return true
}

func (cmd commandConf) RequireAuth() bool {
	return true
}

func (cmd commandConf) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(550, "Action not taken")
}

// commandSize responds to the SIZE FTP command. It returns the size of the
// requested path in bytes.
type commandSize struct{}

func (cmd commandSize) IsExtend() bool {
	return false
}

func (cmd commandSize) RequireParam() bool {
	return true
}

func (cmd commandSize) RequireAuth() bool {
	return true
}

func (cmd commandSize) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	stat, err := conn.driver.Stat(path)
	if err != nil {
		log.Printf("Size: error(%s)", err)
		_, _ = conn.writeMessage(450, fmt.Sprint("path", path, "not found"))
	} else {
		_, _ = conn.writeMessage(213, strconv.Itoa(int(stat.Size())))
	}
}

// commandStor responds to the STOR FTP command. It allows the user to upload a
// new file.
type commandStor struct{}

func (cmd commandStor) IsExtend() bool {
	return false
}

func (cmd commandStor) RequireParam() bool {
	return true
}

func (cmd commandStor) RequireAuth() bool {
	return true
}

func (cmd commandStor) Execute(conn *Conn, param string) {
	targetPath := conn.buildPath(param)
	_, _ = conn.writeMessage(150, "Data transfer starting")

	defer func() {
		conn.appendData = false
	}()

	bytes, err := conn.driver.PutFile(targetPath, conn.dataConn, conn.appendData)
	if err == nil {
		msg := "OK, received " + strconv.Itoa(int(bytes)) + " bytes"
		_, _ = conn.writeMessage(226, msg)
	} else {
		_, _ = conn.writeMessage(450, fmt.Sprint("error during transfer: ", err))
	}
}

type commandStorHercules struct{}

func (cmd commandStorHercules) IsExtend() bool {
	return false
}

func (cmd commandStorHercules) RequireParam() bool {
	return true
}

func (cmd commandStorHercules) RequireAuth() bool {
	return true
}

func (cmd commandStorHercules) Execute(conn *Conn, param string) {
	targetPath := conn.buildPath(param)
	_, err := conn.writeMessageMultiline(150, fmt.Sprintf("Command not yet available\r\n%s", targetPath))
	if err != nil {
		log.Printf("%s", err)
	}
	// TODO check access as unprivileged
	// TODO start hercules to fetch file
}

// commandStru responds to the STRU FTP command.
//
// like the MODE and TYPE commands, stru[cture] dates back to a time when the
// FTP protocol was more aware of the content of the files it was transferring,
// and would sometimes be expected to translate things like EOL markers on the
// fly.
//
// These days files are sent unmodified, and F(ile) mode is the only one we
// really need to support.
type commandStru struct{}

func (cmd commandStru) IsExtend() bool {
	return false
}

func (cmd commandStru) RequireParam() bool {
	return true
}

func (cmd commandStru) RequireAuth() bool {
	return true
}

func (cmd commandStru) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "F" {
		_, _ = conn.writeMessage(200, "OK")
	} else {
		_, _ = conn.writeMessage(504, "STRU is an obsolete command")
	}
}

// commandSyst responds to the SYST FTP command by providing a canned response.
type commandSyst struct{}

func (cmd commandSyst) IsExtend() bool {
	return false
}

func (cmd commandSyst) RequireParam() bool {
	return false
}

func (cmd commandSyst) RequireAuth() bool {
	return true
}

func (cmd commandSyst) Execute(conn *Conn, param string) {
	_, _ = conn.writeMessage(215, "UNIX Type: L8")
}

// commandType responds to the TYPE FTP command.
//
//  like the MODE and STRU commands, TYPE dates back to a time when the FTP
//  protocol was more aware of the content of the files it was transferring, and
//  would sometimes be expected to translate things like EOL markers on the fly.
//
//  Valid options were A(SCII), I(mage), E(BCDIC) or LN (for local type). Since
//  we plan to just accept bytes from the speedtest_client unchanged, I think Image mode is
//  adequate. The RFC requires we accept ASCII mode however, so accept it, but
//  ignore it.
type commandType struct{}

func (cmd commandType) IsExtend() bool {
	return false
}

func (cmd commandType) RequireParam() bool {
	return false
}

func (cmd commandType) RequireAuth() bool {
	return true
}

func (cmd commandType) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "A" {
		_, _ = conn.writeMessage(200, "Type set to ASCII")
	} else if strings.ToUpper(param) == "I" {
		_, _ = conn.writeMessage(200, "Type set to binary")
	} else {
		_, _ = conn.writeMessage(500, "Invalid type")
	}
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (cmd commandUser) IsExtend() bool {
	return false
}

func (cmd commandUser) RequireParam() bool {
	return true
}

func (cmd commandUser) RequireAuth() bool {
	return false
}

func (cmd commandUser) Execute(conn *Conn, param string) {
	conn.reqUser = param
	_, _ = conn.writeMessage(331, "User name ok, password required")
}

// GridFTP Extensions (https://www.ogf.org/documents/GFD.20.pdf)

// Striped Passive
//
// This command is analogous to the PASV command, but allows an array of
// host/port connections to be returned. This enables STRIPING, that is,
// multiple network endpoints (multi-homed hosts, or multiple hosts) to
// participate in the transfer.
type commandSpas struct{}

func (cmd commandSpas) IsExtend() bool {
	return true
}

func (cmd commandSpas) RequireParam() bool {
	return false
}

func (cmd commandSpas) RequireAuth() bool {
	return true
}

func (cmd commandSpas) Execute(conn *Conn, param string) {

	sockets := make([]socket2.DataSocket, conn.parallelism)
	listener := make([]*scion.Listener, conn.parallelism)
	var err error

	line := "Entering Striped Passive Mode\n"

	for i := range listener {
		listener[i], err = conn.NewListener()

		if err != nil {
			_, _ = conn.writeMessage(425, "Data connection failed")

			// Close already opened sockets
			for j := 0; j < i; j++ {
				_ = listener[i].Close()
			}
			return
		}

		// Addr().String() return
		// 1-ff00:0:110,[127.0.0.1]:5848 (UDP)
		// Remove Protocol first
		addr := listener[i].String()

		line += " " + addr + "\r\n"
	}

	_, _ = conn.writeMessageMultiline(229, line)

	for i := range listener {
		connection, _, err := listener[i].Accept()
		if err != nil {
			_, _ = conn.writeMessage(426, "Connection closed, failed to open data connection")

			// Close already opened sockets
			for j := 0; j < i; j++ {
				_ = listener[i].Close()
			}
			return
		}
		sockets[i] = socket2.NewScionSocket(connection)
	}

	conn.dataConn = socket2.NewMultiSocket(sockets, conn.blockSize)

}

type commandEret struct{}

func (commandEret) IsExtend() bool {
	return true
}

func (commandEret) RequireParam() bool {
	return true
}

func (commandEret) RequireAuth() bool {
	return true
}

// TODO: Handle conn.lastFilePos yet
func (commandEret) Execute(conn *Conn, param string) {

	params := strings.Split(param, " ")
	module := strings.Split(params[0], "=")
	moduleName := module[0]
	moduleParams := strings.Split(strings.Trim(module[1], "\""), ",")
	offset, err := strconv.Atoi(moduleParams[0])
	if err != nil {
		_, _ = conn.writeMessage(501, "Failed to parse parameters")
		return
	}
	length, err := strconv.Atoi(moduleParams[1])
	if err != nil {
		_, _ = conn.writeMessage(501, "Failed to parse parameters")
		return
	}
	path := conn.buildPath(params[1])

	if moduleName == mode.PartialFileTransport {

		bytes, data, err := conn.driver.GetFile(path, int64(offset))
		if err != nil {
			_, _ = conn.writeMessage(551, "File not available")
			return
		}

		if length > int(bytes) {
			length = int(bytes)
		}

		buffer := make([]byte, length)
		n, err := data.Read(buffer)
		if n != length || err != nil {
			_, _ = conn.writeMessage(551, "Error reading file")
			return
		}

		defer data.Close()

		_, _ = conn.writeMessage(150, fmt.Sprintf("Data transfer starting %v bytes", bytes))

		conn.sendOutofbandData(buffer)
	} else {
		_, _ = conn.writeMessage(502, "Only PFT supported")
	}
}
