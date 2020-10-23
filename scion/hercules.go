package scion

import (
	"fmt"
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
