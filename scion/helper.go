package scion

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/scionproto/scion/go/lib/snet"
)

func AddrToString(addr *snet.Addr) string {
	if addr.Host == nil {
		return fmt.Sprintf("%s,<nil>", addr.IA)
	}
	return fmt.Sprintf("%s,[%v]", addr.IA, addr.Host.L3)
}

func GetPort(addr net.Addr) (int, error) {
	parts := strings.Split(addr.String(), ":")
	portPart := parts[len(parts)-1]

	// Take Port, which might include the Protocol: 2121 (UDP)
	portProto := strings.Split(portPart, " ")
	port, err := strconv.Atoi(portProto[0])
	if err != nil {
		return -1, err
	}
	return port, nil
}

func ParseAddress(addr string) (host string, port int, err error) {

	splitted := strings.Split(addr, ":")
	if len(splitted) < 2 {
		err = fmt.Errorf("%s is not a valid address with port", addr)
		return
	}

	port, err = strconv.Atoi(splitted[len(splitted)-1])
	if err != nil {
		err = fmt.Errorf("%s should be a number (port)", splitted[len(splitted)-1])
		return
	}

	host = strings.Join(splitted[0:len(splitted)-1], ":")
	return
}
