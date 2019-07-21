package scion

import (
	"encoding/binary"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
	"io"
	"net"
	"strings"
)

type Listener struct {
	quicListener quic.Listener
	local        *snet.Addr
}

func Listen(address string) (*Listener, error) {

	addr, err := snet.AddrFromString(address)
	if err != nil {
		return nil, err
	}

	err = initNetwork(addr)
	if err != nil {
		return nil, err
	}

	listener, err := squic.ListenSCION(nil, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

	return &Listener{
		listener,
		addr,
	}, nil
}

func (listener *Listener) Port() int {
	port, _ := GetPort(listener.Addr())
	return port
}


func (listener *Listener) Addr() net.Addr {
	return listener.local
}

func (listener *Listener) Close() error {
	return listener.quicListener.Close()
}

func (listener *Listener) Accept() (*Connection, error) {

	session, err := listener.quicListener.Accept()
	if err != nil {
		return nil, fmt.Errorf("couldn't accept SQUIC connection: %s", err)
	}

	remote := session.RemoteAddr().String()
	remote = strings.Split(remote, " ")[0]

	remoteAddr, err := snet.AddrFromString(remote)
	if err != nil {
		return nil, err
	}

	stream, err := session.AcceptStream()

	err = receiveHandshake(stream)
	if err != nil {
		return nil, err
	}

	return NewSQuicConnection(stream, listener.local, remoteAddr), nil
}

func receiveHandshake(rw io.ReadWriter) error {

	msg := make([]byte, 1)
	err := binary.Read(rw, binary.BigEndian, msg)
	if err != nil {
		return err
	}

	return nil
}
