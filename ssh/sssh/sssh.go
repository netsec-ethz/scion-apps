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

package sssh

import (
	"errors"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
)

// DialSCION starts a client connection to the given SSH server over SCION using QUIC.
func DialSCION(addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	transportStream, err := quicconn.Dial(addr)
	if err != nil {
		return nil, err
	}
	return newSSHClient(transportStream, config)
}

// DialSCION starts a client connection to the given SSH server over SCION using QUIC
// Passes an instance of PathAppConf to the connection to make it aware of user-defined path configurations
func DialSCIONWithConf(addr string, config *ssh.ClientConfig, appConf *scionutils.PathAppConf) (*ssh.Client, error) {
	raddr, err := appnet.ResolveUDPAddr(addr)
	if err != nil {
		return nil, err
	}
	sconn, err := appnet.Listen(nil)
	if err != nil {
		return nil, err
	}
	policyConn := scionutils.NewPolicyConn(sconn, appConf)
	transportStream, err := quicconn.New(policyConn, raddr)
	if err != nil {
		return nil, err
	}

	return newSSHClient(transportStream, config)
}

// newSSHClient creates a new ssh ClientConn and with that a new ssh.Client
func newSSHClient(transportStream net.Conn, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, nc, rc, err := ssh.NewClientConn(transportStream, transportStream.RemoteAddr().String(), config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, nc, rc), nil
}

// TunnelDialSCION creates a tunnel using the given SSH client.
func TunnelDialSCION(client *ssh.Client, addr string) (net.Conn, error) {
	openChannelData := directSCIONData{
		addr,
	}

	c, requests, err := (*client).OpenChannel("direct-scionquic", ssh.Marshal(&openChannelData))
	if err != nil {
		return nil, err
	}

	go ssh.DiscardRequests(requests)

	return &chanConn{
		c,
	}, err
}

type directSCIONData struct {
	addr string
}

// chanConn fulfills the net.Conn interface without having to hold laddr or raddr.
type chanConn struct {
	ssh.Channel
}

// LocalAddr returns the local network address.
func (t *chanConn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
}

// RemoteAddr returns the remote network address.
func (t *chanConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
}

// SetDeadline sets the read and write deadlines associated
// with the connection.
func (t *chanConn) SetDeadline(deadline time.Time) error {
	return errors.New("scion-ssh: deadline not supported")
}

// SetReadDeadline sets the read deadline.
// A zero value for t means Read will not time out.
// After the deadline, the error from Read will implement net.Error
// with Timeout() == true.
func (t *chanConn) SetReadDeadline(deadline time.Time) error {
	return errors.New("scion-ssh: deadline not supported")
}

// SetWriteDeadline exists to satisfy the net.Conn interface
// but is not implemented by this type.  It always returns an error.
func (t *chanConn) SetWriteDeadline(deadline time.Time) error {
	return errors.New("scion-ssh: deadline not supported")
}
