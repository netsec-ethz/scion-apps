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
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// Shell opens a new Shell session on the server this Client is connected to.
func (client *Client) Shell() error {
	var (
		termWidth, termHeight = 80, 24
	)

	client.session.Stdout = os.Stdout
	client.session.Stderr = os.Stderr
	client.session.Stdin = os.Stdin

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}

		defer func() {
			_ = term.Restore(fd, oldState)
		}()

		w, h, err := term.GetSize(fd)
		if err == nil {
			termWidth = w
			termHeight = h
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

	if err := client.WaitSession(); err != nil {
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
		_, _ = session.SendRequest("window-change", false, termSize(fd))
	}
}

func termSize(fd uintptr) []byte {
	size := make([]byte, 16)

	width, height, err := term.GetSize(int(fd))
	if err != nil {
		binary.BigEndian.PutUint32(size, uint32(80))
		binary.BigEndian.PutUint32(size[4:], uint32(24))
		return size
	}

	binary.BigEndian.PutUint32(size, uint32(width))
	binary.BigEndian.PutUint32(size[4:], uint32(height))

	return size
}
