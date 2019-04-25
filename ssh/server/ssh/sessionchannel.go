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

func handleSession(newChannel ssh.NewChannel) {
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Debug("Could not accept channel (%s)", err)
		return
	}

	var bashf *os.File

	close := func() {
		err = connection.Close()
		if err != nil {
			log.Debug("Could not close connection: %v", err)
		}
	}

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func() {
		var once sync.Once
		defer once.Do(close)
		for req := range requests {
			switch req.Type {
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				} else {
					log.Debug("Non-empty shell payload not yet supported!")
				}
			case "pty-req":

				bash := exec.Command("bash")

				ptyClose := func() {
					bash.Process.Kill()
					err := bash.Wait()
					if err != nil {
						log.Debug("Error waiting for bash to end: %v", err)
					}

					tb := []byte{0, 0, 0, 0}
					connection.SendRequest("exit-status", false, tb)

					close()

					log.Debug("Session closed")
				}

				// Allocate a terminal for this channel
				log.Debug("Creating pty...")
				bashf, err = pty.Start(bash)
				if err != nil {
					log.Debug("Could not start pty (%s)", err)
					return
				}

				//pipe session to bash and visa-versa
				go func() {
					io.Copy(connection, bashf)
					once.Do(ptyClose)
				}()
				go func() {
					io.Copy(bashf, connection)
					once.Do(ptyClose)
				}()

				termLen := req.Payload[3]
				w, h := parseDims(req.Payload[termLen+4:])
				SetWinsize(bashf.Fd(), w, h)
				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
			case "window-change":
				if bashf == nil {
					log.Debug("No pty requested!")
				} else {
					w, h := parseDims(req.Payload)
					SetWinsize(bashf.Fd(), w, h)
				}
			case "exec":
				cmdStrLen := binary.BigEndian.Uint32(req.Payload[0:4])
				cmdStr := string(req.Payload[4 : cmdStrLen+4])
				cmd := exec.Command("bash", "-c", cmdStr)

				stdin, err := cmd.StdinPipe()
				if err != nil {
					log.Debug("Error creating stdin pipe: %v", err)
					continue
				}

				stdout, err := cmd.StdoutPipe()
				if err != nil {
					log.Debug("Error creating stdout pipe: %v", err)
					continue
				}

				stderr, err := cmd.StderrPipe()
				if err != nil {
					log.Debug("Error creating stderr pipe: %v", err)
					continue
				}

				var pipesWait sync.WaitGroup

				// we want to wait for stdout and stderr before closing connection, but don't really mind about stdin
				pipesWait.Add(2)
				go func() {
					io.Copy(stdin, connection)
				}()
				go func() {
					io.Copy(connection, stdout)
					pipesWait.Done()
				}()
				go func() {
					io.Copy(connection.Stderr(), stderr)
					pipesWait.Done()
				}()

				err = cmd.Start()
				if err != nil {
					log.Debug("Error running command %s: %v", cmdStr, err)
					continue
				}

				go func() {
					err := cmd.Wait()
					if err != nil {
						log.Debug("Error waiting for command %s to end: %v", cmdStr, err)
					}

					pipesWait.Wait()

					tb := []byte{0, 0, 0, 0}
					connection.SendRequest("exit-status", false, tb)

					once.Do(close)
				}()

				err = req.Reply(true, nil)
				if err != nil {
					log.Debug("Error responding to request: %v", err)
					continue
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
