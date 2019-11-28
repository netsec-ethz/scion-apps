package sssh

import (
	"errors"
	"github.com/netsec-ethz/scion-apps/ssh/appconf"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
)

// DialSCION starts a client connection to the given SSH server over SCION using QUIC.
func DialSCION(clientAddr string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	transportStream, err := scionutils.DialSCION(clientAddr, addr)

	if err != nil {
		return nil, err
	}

	conn, nc, rc, err := ssh.NewClientConn(transportStream, transportStream.RemoteAddr().String(), config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, nc, rc), nil
}
// DialSCION starts a client connection to the given SSH server over SCION using QUIC.
func DialSCIONWithConf(clientAddr string, addr string, config *ssh.ClientConfig, appConf *appconf.AppConf) (*ssh.Client, error) {
	transportStream, err := scionutils.DialSCIONWithConf(clientAddr, addr, appConf)

	if err != nil {
		return nil, err
	}

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
