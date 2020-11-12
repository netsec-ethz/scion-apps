package scion

import (
	"crypto/tls"
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"net"
)

func FindInterfaceName(localAddr net.IP) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("could not retrieve network interfaces: %s", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("could not get interface addresses: %s", err)
		}

		if iface.Flags&net.FlagUp == 0 {
			continue // interface not up
		}

		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if ok && ip.IP.To4() != nil && ip.IP.To4().Equal(localAddr) {
				return iface.Name, nil
			}
		}
	}

	return "", fmt.Errorf("could not find interface with address %s", localAddr)
}

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
