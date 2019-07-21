package scion

import (
	"fmt"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

func initNetwork(local *snet.Addr) error {

	if snet.DefNetwork == nil {

		err := initSciond(local)
		if err != nil {
			return fmt.Errorf("failed to initialize SCION: %s", err)
		}
	}

	err := squic.Init("", "")
	if err != nil {
		return fmt.Errorf("failed to initilaze SQUIC: %s", err)
	}

	return nil
}

func initSciond(local *snet.Addr) error {
	sock := sciond.GetDefaultSCIONDPath(nil)
	dispatcher := ""

	// Try with default socket
	err := snet.Init(local.IA, sock, dispatcher)
	if err == nil {
		return nil
	}

	// Try with socket for IA
	// Required when used in local topology with multiple sockets
	sock = sciond.GetDefaultSCIONDPath(&local.IA)
	return snet.Init(local.IA, sock, dispatcher)
}
