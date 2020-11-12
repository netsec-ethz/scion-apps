package scion

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scionproto/scion/go/lib/snet"
)

func ParseAddress(addr string) (host string, port uint16, err error) {

	splitted := strings.Split(addr, ":")
	if len(splitted) < 2 {
		err = fmt.Errorf("%s is not a valid address with port", addr)
		return
	}

	var _port uint64
	_port, err = strconv.ParseUint(splitted[len(splitted)-1], 10, 16)
	if err != nil {
		err = fmt.Errorf("%s should be a number (port)", splitted[len(splitted)-1])
		return
	}
	port = uint16(_port)

	host = strings.Join(splitted[0:len(splitted)-1], ":")
	return
}

// Parses addresses that contain transport layer information, e.g. (UDP)
func ParseCompleteAddress(addr string) (host string, port uint16, err error) {
	result := strings.Split(addr, " ")
	if len(result) < 1 {
		return "", 0, fmt.Errorf("failed to parse address: %s", addr)
	}
	return ParseAddress(result[0])
}

// Should be treated immutable
type Address struct {
	host string
	port uint16
	addr snet.UDPAddr
}

func ConvertAddress(addr string) (Address, error) {
	parsed, err := snet.ParseUDPAddr(addr)
	if err != nil {
		return Address{}, fmt.Errorf("%s is not a valid address: %s", addr, err)
	}

	splitted := strings.Split(addr, ":")
	if len(splitted) < 2 {
		return Address{}, fmt.Errorf("%s is not a valid address with port", addr)
	}

	_port, err := strconv.ParseUint(splitted[len(splitted)-1], 10, 16)
	if err != nil {
		return Address{}, fmt.Errorf("%s should be a number (port)", splitted[len(splitted)-1])
	}
	port := uint16(_port)

	host := strings.Join(splitted[0:len(splitted)-1], ":")

	return Address{host, port, *parsed}, nil
}

func (addr Address) Port() uint16 {
	return addr.port
}

func (addr Address) Host() string {
	return addr.host
}

func (addr Address) Addr() snet.UDPAddr {
	return addr.addr
}

func (addr Address) String() string {
	return addr.host + ":" + strconv.Itoa(int(addr.port))
}

func (addr Address) Network() string {
	return "???"
}
