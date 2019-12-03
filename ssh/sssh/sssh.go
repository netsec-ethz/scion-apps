package sssh

import (
	"errors"
	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
)

// DialSCION starts a client connection to the given SSH server over SCION using QUIC.
func DialSCION(clientAddr string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return dialSCION(clientAddr, addr, config, scionutils.DialSCION)
}
// DialSCION starts a client connection to the given SSH server over SCION using QUIC
// Passes an instance of PathAppConf to the connection to make it aware of user-defined path configurations
func DialSCIONWithConf(clientAddr string, addr string, config *ssh.ClientConfig, appConf *scionutils.PathAppConf) (*ssh.Client, error) {
	return dialSCION(clientAddr, addr, config, dialCSCIONWithConf(appConf))
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

type dial func (localAddress string, remoteAddress string) (*quicconn.QuicConn, error)

func dialSCION(clientAddr string, addr string, config *ssh.ClientConfig, dialFunc dial) (*ssh.Client, error) {
	transportStream, err := dialFunc(clientAddr, addr)
	if err != nil {
		return nil, err
	}

	conn, nc, rc, err := ssh.NewClientConn(transportStream, transportStream.RemoteAddr().String(), config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, nc, rc), nil
}
func dialCSCIONWithConf(conf *scionutils.PathAppConf) dial {
	return func(localAddress string, remoteAddress string) (conn *quicconn.QuicConn, e error) {
		return scionutils.DialSCIONWithConf(localAddress, remoteAddress, conf)
	}
}
