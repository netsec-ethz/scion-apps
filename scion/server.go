package scion

import (
	"fmt"
	"net"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

func Listen(address string) (net.Listener, error) {

	fmt.Println(address)

	addr, _ := snet.AddrFromString(address)

	err := initNetwork(*addr)
	if err != nil {
		return nil, err
	}

	listener, err := squic.ListenSCION(nil, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

	return &Listener{
		listener,
		addr,
	}, nil
}
