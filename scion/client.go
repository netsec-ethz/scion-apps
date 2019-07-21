package scion

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

func Dial(local, remote *snet.Addr) (*Connection, error) {

	err := initNetwork(local)
	if err != nil {
		return nil, err
	}

	err = setupPath(local, remote)
	if err != nil {
		return nil, err
	}

	session, err := squic.DialSCION(nil, local, remote, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to dial %s: %s", AddrToString(remote), err)
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

	local, err := snet.AddrFromString(localAddr)
	if err != nil {
		return nil, err
	}

	remote, err := snet.AddrFromString(remoteAddr)
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
