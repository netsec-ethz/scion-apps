package scion

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/lucas-clemente/quic-go"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Listener struct {
	quicListener quic.Listener
	Address
}

func Listen(address string) (*Listener, error) {
	addr, err := ConvertAddress(address)
	if err != nil {
		return nil, err
	}

	err = initNetwork(addr)
	if err != nil {
		return nil, err
	}

	tmpAddr := addr.Addr()
	listener, err := squic.ListenSCION(nil, &tmpAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

	_, port, err := ParseCompleteAddress(listener.Addr().String())
	if err != nil {
		return nil, err
	}

	addr.port = port

	return &Listener{
		listener,
		addr,
	}, nil
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

	remoteAddr, err := ConvertAddress(remote)
	if err != nil {
		return nil, err
	}

	stream, err := session.AcceptStream()

	err = receiveHandshake(stream)
	if err != nil {
		return nil, err
	}

	return NewSQuicConnection(stream, listener.Address, remoteAddr), nil
}

func receiveHandshake(rw io.ReadWriter) error {

	msg := make([]byte, 1)
	err := binary.Read(rw, binary.BigEndian, msg)
	if err != nil {
		return err
	}

	return nil
}
