package scion

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/scionproto/scion/go/lib/snet/squic"
)

func Dial(local, remote Address) (*Connection, error) {

	err := initNetwork(local)
	if err != nil {
		return nil, err
	}

	err = setupPath(local.Addr(), remote.Addr())
	if err != nil {
		return nil, err
	}

	l := local.Addr()
	r := remote.Addr()
	session, err := squic.DialSCION(nil, &l, &r, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to dial %s: %s", remote, err)
	}

	stream, err := session.OpenStream()
	if err != nil {
		return nil, fmt.Errorf("unable to open stream: %s", err)
	}

	err = sendHandshake(stream)
	if err != nil {
		return nil, err
	}

	return NewSQuicConnection(stream, local, remote), nil
}

func DialAddr(localAddr, remoteAddr string) (*Connection, error) {

	local, err := ConvertAddress(localAddr)
	if err != nil {
		return nil, err
	}

	remote, err := ConvertAddress(remoteAddr)
	if err != nil {
		return nil, err
	}

	return Dial(local, remote)
}

func sendHandshake(rw io.ReadWriter) error {

	msg := []byte{200}

	binary.Write(rw, binary.BigEndian, msg)

	// log.Debug("Sent handshake", "msg", msg)

	return nil
}
