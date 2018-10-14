package shttp

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
	"gihtub.com/chaehni/scion-http/utils/utils"
)

type Server struct {
	AddrString String
	Addr       snet.Addr
}

func (srv *Server) ListenAndServe() {

	srv.initSCIONConnection()

	li, err := squic.ListenSCION(nil, srv.Addr)
	defer li.Close()

	if err != nil {
		log.Fatal("Failed to listen on %v: %v", srv.Addr, err)
	}

	for {
		c, err := li.Accept()
		defer c.Close()

		if err != nil {
			log.Printf(err.Error())
		}

		bs, err := ioutil.ReadAll(c)
		if err != nil {
			log.Printf(err.Error())
		}
		fmt.Println(string(bs))
	}
}

func (srv *Server) initSCIONConnection(serverAddress, tlsCertFile, tlsKeyFile string) (*snet.Addr, error) {

	log.Println("Initializing SCION connection")

	Srv.Addr, err := snet.AddrFromString(srv.AddrString)
	if err != nil {
		return nil, err
	}

	err = snet.Init(srv.Addr.IA, utils.GetSciondAddr(srv.Addr), utils.GetDispatcherAddr(srv.Addr))
	if err != nil {
		return srv.Addr, err
	}

	err = squic.Init(tlsKeyFile, tlsCertFile)
	if err != nil {
		return serverCCAddr, err
	}

	return serverCCAddr, nil

}
