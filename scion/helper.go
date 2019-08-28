package scion

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scionproto/scion/go/lib/snet"
)

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

// Parses addresses that contain transport layer information, e.g. (UDP)
func ParseCompleteAddress(addr string) (host string, port int, err error) {
	result := strings.Split(addr, " ")
	if len(result) < 1 {
		return "", -1, fmt.Errorf("failed to parse address: %s", addr)
	}
	return ParseAddress(result[0])
}

// Should be treated immutable
type Address struct {
	host string
	port int
	addr snet.Addr
}

func ConvertAddress(addr string) (Address, error) {
	parsed, err := snet.AddrFromString(addr)
	if err != nil {
		return Address{}, fmt.Errorf("%s is not a valid address: %s", addr, err)
	}

	splitted := strings.Split(addr, ":")
	if len(splitted) < 2 {
		return Address{}, fmt.Errorf("%s is not a valid address with port", addr)
	}

	port, err := strconv.Atoi(splitted[len(splitted)-1])
	if err != nil {
		return Address{}, fmt.Errorf("%s should be a number (port)", splitted[len(splitted)-1])
	}

	host := strings.Join(splitted[0:len(splitted)-1], ":")

	return Address{host, port, *parsed}, nil
}

func (addr Address) Port() int {
	return addr.port
}

func (addr Address) Host() string {
	return addr.host
}

func (addr Address) Addr() snet.Addr {
	return addr.addr
}

func (addr Address) String() string {
	return addr.host + ":" + strconv.Itoa(addr.port)
}

func (addr Address) Network() string {
	return "???"
}
