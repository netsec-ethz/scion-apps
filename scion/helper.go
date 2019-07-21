package scion

import (
	"fmt"
	"github.com/scionproto/scion/go/lib/snet"
)

func AddrToString(addr *snet.Addr) string {
	if addr.Host == nil {
		return fmt.Sprintf("%s,<nil>", addr.IA)
	}
	return fmt.Sprintf("%s,[%v]", addr.IA, addr.Host.L3)
}