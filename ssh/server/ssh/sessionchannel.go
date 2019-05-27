package ssh

import (
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	log "github.com/inconshreveable/log15"
	"github.com/kr/pty"

	"golang.org/x/crypto/ssh"
)

func handleSession(perms *ssh.Permissions, newChannel ssh.NewChannel) {
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Error("Could not accept channel", "error", err)
		return
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
		close := func() {
			cmd.Process.Kill()
			err := cmd.Wait()
			if err != nil {
				log.Error("Error waiting for bash to end", "error", err)
			}

			tb := []byte{0, 0, 0, 0}
			connection.SendRequest("exit-status", false, tb)

			once.Do(closeConn)

			log.Debug("Session closed")
		}

		var pipesWait sync.WaitGroup
		pipesWait.Add(2)
		go func() {
			pipesWait.Wait()
			close()
		}()

		if hasRequestedPty {
			log.Debug("Creating pty...")
			cmdf, err = pty.Start(cmd)
			if err != nil {
				return err
			}

			var once sync.Once
			go func() {
				io.Copy(connection, cmdf)
				pipesWait.Done()
			}()
			go func() {
				io.Copy(cmdf, connection)
				pipesWait.Done()
			}()

			termLen := ptyPayload[3]
			w, h := parseDims(ptyPayload[termLen+4:])
			SetWinsize(cmdf.Fd(), w, h)
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
			switch req.Type {
			case "shell":
				// TODO determine and use default shell, don't force bash
				err := execCmd("bash")
				if err != nil {
					log.Error("Can't create shell!", "error", err)
				}

				if req.WantReply {
					req.Reply(true, nil)
				}
			case "pty-req":
				hasRequestedPty = true
				ptyPayload = req.Payload
				if req.WantReply {
					req.Reply(true, nil)
				}
			case "window-change":
				if cmdf == nil {
					log.Debug("Tried to change window size but no pty requested!")
				} else {
					w, h := parseDims(req.Payload)
					SetWinsize(cmdf.Fd(), w, h)
					if req.WantReply {
						req.Reply(true, nil)
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
					req.Reply(true, nil)
				}
			default:
				log.Debug("Unkown session request type %s", req.Type)
			}
		}
	}()
}

// parseDims extracts terminal dimensions (width x height) from the provided buffer.
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// ======================

// Winsize stores the Height and Width of a terminal.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

// SetWinsize sets the size of the given pty.
func SetWinsize(fd uintptr, w, h uint32) {
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}
