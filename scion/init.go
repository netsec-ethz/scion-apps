package scion

import (
	"context"
	"fmt"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"sync"

	"github.com/scionproto/scion/go/lib/sciond"
)

var initialize sync.Once
var defNetwork *snet.SCIONNetwork

const (
	KEYPATH = "/etc/scion/gen-certs/tls.key"
	PEMPATH = "/etc/scion/gen-certs/tls.pem"
)

func initNetwork(local Address) (*snet.SCIONNetwork, error) {
	log.Setup(log.Config{})

	var err error
	initialize.Do(func() {
		var (
			sciondConn sciond.Connector
			localIA    addr.IA
		)
		sciondConn, err = initSciond(local)
		if err != nil {
			err = fmt.Errorf("failed to initialize SCION: %s", err)
			return
		}

		localIA, err = sciondConn.LocalIA(context.Background())
		if err != nil {
			return
		}
		pathQuerier := sciond.Querier{Connector: sciondConn, IA: localIA}
		defNetwork = snet.NewNetworkWithPR(
			localIA,
			reliable.NewDispatcher(reliable.DefaultDispPath),
			pathQuerier,
			sciond.RevHandler{Connector: sciondConn},
		)
	})

	return defNetwork, err
}

func initSciond(local Address) (sciond.Connector, error) {
	lcl := local.Addr()

	// Try with default socket
	sciondConn, err := sciond.NewService(sciond.DefaultSCIONDAddress).Connect(context.Background())
	if err == nil {
		return sciondConn, nil
	}

	// Try with socket for IA
	// Required when used in lcl topology with multiple sockets
	return sciond.NewService(sciond.GetDefaultSCIONDAddress(&lcl.IA)).Connect(context.Background())
}
