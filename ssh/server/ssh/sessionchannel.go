// Copyright 2020 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	log "github.com/inconshreveable/log15"
	"golang.org/x/crypto/ssh"
)

func handleSession(perms *ssh.Permissions, newChannel ssh.NewChannel) error {
	connection, requests, err := newChannel.Accept()
	if err != nil {
		return fmt.Errorf("could not accept channel: %w", err)
	}

	closeConn := func() {
		err = connection.Close()
		if err != nil {
			log.Error("Could not close connection", "error", err)
			return
		}

		log.Debug("Connection closed")
	}
	var once sync.Once

	var cmdf *os.File
	hasRequestedPty := false
	var ptyPayload []byte

	execCmd := func(name string, arg ...string) error {
		cmd := exec.Command(name, arg...)
		username, ok := perms.CriticalOptions["user"]
		if !ok {
			return errors.New("missing criticla option user")
		}
		currentuser, err := user.Current()
		if err != nil || username != currentuser.Username {
			usr, err := user.Lookup(username)
			if err != nil {
				return err
			}
			uid, err := strconv.ParseUint(usr.Uid, 10, 32)
			if err != nil {
				return err
			}
			gid, err := strconv.ParseUint(usr.Gid, 10, 32)
			if err != nil {
				return err
			}
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			cmd.SysProcAttr.Credential = &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			}
		}
		close := func() {
			if err := cmd.Process.Kill(); err != nil {
				log.Error("Error killing bash", "error", err)
			}
			if err := cmd.Wait(); err != nil {
				log.Error("Error waiting for bash to end", "error", err)
			}

			tb := []byte{0, 0, 0, 0}
			_, err := connection.SendRequest("exit-status", false, tb)
			if err != nil {
				log.Error("Error sending exit status", "error", err)
			}

			once.Do(closeConn)

			log.Debug("Session closed")
		}

		if hasRequestedPty {
			log.Debug("Creating pty...")
			cmdf, err = pty.Start(cmd)
			if err != nil {
				return err
			}

			var once sync.Once
			go func() {
				_, err := io.Copy(connection, cmdf)
				log.Debug("Pty to connection copy ended", "error", err)
				once.Do(close)
			}()
			go func() {
				_, err := io.Copy(cmdf, connection)
				log.Debug("Connection to pty copy ended", "error", err)
				once.Do(close)
			}()

			termLen := ptyPayload[3]
			w, h := parseDims(ptyPayload[termLen+4:])
			if err := setWinsize(cmdf.Fd(), w, h); err != nil {
				log.Debug("Error trying to change window size", "error", err, "w", w, "h", h)
			}
		} else {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return err
			}

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return err
			}

			stderr, err := cmd.StderrPipe()
			if err != nil {
				return err
			}

			// we want to wait for stdout and stderr before closing connection, but don't really mind about stdin
			var pipesWait sync.WaitGroup
			pipesWait.Add(2)
			go func() {
				pipesWait.Wait()
				close()
			}()

			go func() {
				_, err := io.Copy(stdin, connection)
				log.Debug("Stdin copy ended", "error", err)
			}()
			go func() {
				_, err := io.Copy(connection, stdout)
				log.Debug("Stdout copy ended", "error", err)
				pipesWait.Done()
			}()
			go func() {
				_, err := io.Copy(connection.Stderr(), stderr)
				log.Debug("Stderr copy ended", "error", err)
				pipesWait.Done()
			}()

			err = cmd.Start()
			if err != nil {
				return err
			}
		}

		return nil
	}

	// Sessions have out-of-band requests such as "shell", "pty-req" and "exec"
	go func() {
		defer once.Do(closeConn)
		for req := range requests {
			var replyErr error
			switch req.Type {
			case "shell":
				// TODO determine and use default shell, don't force bash
				err := execCmd("bash")
				if err != nil {
					log.Error("Can't create shell!", "error", err)
				}
				if req.WantReply {
					replyErr = req.Reply(err != nil, nil)
				}
			case "pty-req":
				hasRequestedPty = true
				ptyPayload = req.Payload
				if req.WantReply {
					replyErr = req.Reply(true, nil)
				}
			case "window-change":
				if cmdf == nil {
					log.Debug("Tried to change window size but no pty requested!")
				} else {
					w, h := parseDims(req.Payload)
					err := setWinsize(cmdf.Fd(), w, h)
					if err != nil {
						log.Debug("Error trying to change window size", "error", err, "w", w, "h", h)
					}
					if req.WantReply {
						replyErr = req.Reply(err != nil, nil)
					}
				}
			case "exec":
				cmdStrLen := binary.BigEndian.Uint32(req.Payload[0:4])
				cmdStr := string(req.Payload[4 : cmdStrLen+4])
				err := execCmd("bash", "-c", cmdStr)
				if err != nil {
					log.Error("Can't create shell!", "error", err)
				}
				if req.WantReply {
					replyErr = req.Reply(err != nil, nil)
				}
			default:
				log.Debug("Unknown session request type %s", req.Type)
			}
			if replyErr != nil {
				log.Debug("Error replying to request", "error", replyErr, "request", req)
			}
		}
	}()
	return nil
}

// parseDims extracts terminal dimensions (width x height) from the provided buffer.
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// ======================

// winsize stores the Height and Width of a terminal.
type winsize struct {
	Height uint16
	Width  uint16
}

// setWinsize sets the size of the given pty.
func setWinsize(fd uintptr, w, h uint32) error {
	ws := &winsize{Width: uint16(w), Height: uint16(h)}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
	return err
}
