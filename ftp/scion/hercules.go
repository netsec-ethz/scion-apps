package scion

import (
	"crypto/tls"
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

func AllocateUDPPort(remoteAddress string) (uint16, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"scionftp"},
	}

	session, err := appquic.Dial(remoteAddress, tlsConfig, nil)
	if err != nil {
		return 0, fmt.Errorf("unable to dial %s: %s", remoteAddress, err)
	}

	stream, err := session.OpenStream()
	if err != nil {
		return 0, fmt.Errorf("unable to open stream: %s", err)
	}

	_, port, err := ParseCompleteAddress(session.LocalAddr().String())
	if err != nil {
		return 0, err
	}

	err = sendHandshake(stream)
	if err != nil {
		return 0, err
	}

	err = stream.Close()
	if err != nil {
		return 0, err
	}

	return port, nil
}
