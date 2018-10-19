package shttp

import (
	"fmt"
	"log"

	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
)

type Client struct {
	AddrString string
	Addr       *snet.Addr
}

func (c *Client) initSCIONConnection(serverAddress string) (*snet.Addr, *snet.Addr, error) {

	log.Println("Initializing SCION connection")

	srvAddr, err := snet.AddrFromString(serverAddress)
	if err != nil {
		return nil, nil, err
	}

	c.Addr, err = snet.AddrFromString(c.AddrString)
	if err != nil {
		return nil, nil, err
	}

	err = snet.Init(c.Addr.IA, utils.GetSCIOND(), utils.GetDispatcher())
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to initialize SCION network:", err)
	}

	log.Println("Initialized SCION network")

	return srvAddr, c.Addr, nil

}
