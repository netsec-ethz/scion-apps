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
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return -1, err
	}
	return port, nil
}

func RemovePort(addr *snet.Addr) string {
	parts := strings.Split(addr.String(), ":")
	return strings.Join(parts[0:len(parts)-1], ":")
}

func ReplacePort(addr *snet.Addr, port int) (*snet.Addr, error) {
	parts := strings.Split(addr.String(), ":")
	parts[len(parts)-1] = strconv.Itoa(port)
	return snet.AddrFromString(strings.Join(parts, ":"))
}
