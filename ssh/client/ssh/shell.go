package ssh

import (
	"encoding/binary"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"

	"github.com/docker/docker/pkg/term"
)

func (client *SSHClient) Shell() error {
	var (
		termWidth, termHeight = 80, 24
	)

	client.session.Stdout = os.Stdout
	client.session.Stderr = os.Stderr
	client.session.Stdin = os.Stdin

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	fd := os.Stdin.Fd()

	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}

		defer term.RestoreTerminal(fd, oldState)

		winsize, err := term.GetWinsize(fd)
		if err == nil {
			termWidth = int(winsize.Width)
			termHeight = int(winsize.Height)
		}
	}

	if err := client.session.RequestPty("xterm", termHeight, termWidth, modes); err != nil {
		return err
	}

	if err := client.session.Shell(); err != nil {
		return err
	}

	// monitor for sigwinch
	go monWinCh(client.session, os.Stdout.Fd())

	err := client.Wait()
	if err != nil {
		return err
	}

	return nil
}

func monWinCh(session *ssh.Session, fd uintptr) {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGWINCH)
	defer signal.Stop(sigs)

	// resize the tty if any signals received
	for range sigs {
		session.SendRequest("window-change", false, termSize(fd))
	}
}

func termSize(fd uintptr) []byte {
	size := make([]byte, 16)

	winsize, err := term.GetWinsize(fd)
	if err != nil {
		binary.BigEndian.PutUint32(size, uint32(80))
		binary.BigEndian.PutUint32(size[4:], uint32(24))
		return size
	}

	binary.BigEndian.PutUint32(size, uint32(winsize.Width))
	binary.BigEndian.PutUint32(size[4:], uint32(winsize.Height))

	return size
}
