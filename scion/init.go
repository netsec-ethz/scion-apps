package scion

import (
	"fmt"
	"sync"

	"github.com/scionproto/scion/go/lib/snet/squic"

	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
)

var initialize sync.Once

func initNetwork(local Address) error {
	var err error
	initialize.Do(func() {
		if snet.DefNetwork == nil {

			err := initSciond(local)
			if err != nil {
				err = fmt.Errorf("failed to initialize SCION: %s", err)
				return
			}
		}

		err := squic.Init("", "")
		if err != nil {
			err = fmt.Errorf("failed to initilaze SQUIC: %s", err)
			return
		}

	})

	return err
}

func initSciond(local Address) error {
	lcl := local.Addr()

	sock := sciond.GetDefaultSCIONDPath(nil)
	dispatcher := ""

	// Try with default socket
	err := snet.Init(lcl.IA, sock, dispatcher)
	if err == nil {
		return nil
	}

	// Try with socket for IA
	// Required when used in lcl topology with multiple sockets
	sock = sciond.GetDefaultSCIONDPath(&lcl.IA)
	return snet.Init(lcl.IA, sock, dispatcher)
}
