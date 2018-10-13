package utils

import (
	"fmt"
	"regexp"

	"github.com/scionproto/scion/go/lib/snet"
)

var addressPortSplitRegex, _ = regexp.Compile(`(.*,\[.*\]):(\d+)`)

func GetSciondAddr(scionAddr *snet.Addr) string {
	return fmt.Sprintf("/run/shm/sciond/sd%d-%d.sock", scionAddr.IA.I, scionAddr.IA.A)
}

func GetDispatcherAddr(scionAddr *snet.Addr) string {
	return "/run/shm/dispatcher/default.sock"
}

func SplitHostPort(hostport string) (host, port string, err error) {
	split := addressPortSplitRegex.FindAllStringSubmatch(hostport, -1)
	if len(split) == 1 {
		return split[0][1], split[0][2], nil
	} else {
		// Shouldn't happen
		return "", "", fmt.Errorf("Invalid SCION address provided")
	}
}
