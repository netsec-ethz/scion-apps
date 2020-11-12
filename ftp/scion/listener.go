package scion

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

type Listener struct {
	quicListener quic.Listener
	Address
}

func Listen(address string, cert *tls.Certificate) (*Listener, error) {
	addr, err := ConvertAddress(address)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		NextProtos:   []string{"scionftp"},
		Certificates: []tls.Certificate{*cert},
	}

	listener, err := appquic.ListenPort(addr.Port(), tlsConfig, nil)
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

func (listener *Listener) Accept() (*Connection, *quic.Session, error) {

	session, err := listener.quicListener.Accept(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't accept APPQUIC connection: %s", err)
	}

	remote := session.RemoteAddr().String()
	remote = strings.Split(remote, " ")[0]

	remoteAddr, err := ConvertAddress(remote)
	if err != nil {
		return nil, nil, err
	}

	stream, err := AcceptStream(&session)
	if err != nil {
		return nil, nil, err
	}

	return NewAppQuicConnection(stream, listener.Address, remoteAddr), &session, nil
}

func AcceptStream(session *quic.Session) (quic.Stream, error) {
	stream, err := (*session).AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}

	err = receiveHandshake(stream)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func receiveHandshake(rw io.ReadWriter) error {

	msg := make([]byte, 1)
	err := binary.Read(rw, binary.BigEndian, msg)
	if err != nil {
		return err
	}

	return nil
}
