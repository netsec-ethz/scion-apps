package utils;

import (
    "log"

    "github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sciond"
)


func getDispatcherAddr(scionAddr *snet.Addr)(string){
    return "/run/shm/dispatcher/default.sock"
}

func InitSCIONConnection(scionAddressString string)(*snet.Addr, error){
    log.Println("Initializing SCION connection")

    scionAddress, err := snet.AddrFromString(scionAddressString)
    if err != nil {
        return nil, err
    }

    err = snet.Init(scionAddress.IA, sciond.GetDefaultSCIONDPath(nil),
    	getDispatcherAddr(scionAddress))
    if err != nil {
        return scionAddress, err
    }

    return scionAddress, nil
}

