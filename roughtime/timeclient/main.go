package main

import (
    "os"
    "log"
    "time"
    "fmt"


    "gopkg.in/alecthomas/kingpin.v2"
    "github.com/perrig/scionlab/roughtime/utils"
    "github.com/perrig/scionlab/roughtime/timeclient/lib"
    "roughtime.googlesource.com/go/client/monotime"
)

var (
    app = kingpin.New("timeclient", "SCION roughtime client")

    clientAddress = app.Flag("address", "Client's SCION address").Required().String()
    chainFile = app.Flag("chain-file", "Name of a file in which the query chain will be maintained").Default("query-chain.json").String()
    maxChainSize = app.Flag("max-chain-size", "Maximum number of entries in chain file").Default("128").Int()
    serversFile = app.Flag("servers", "Name of the file with server configuration").Default("servers.json").String()
)

const (
    defaultServerQuorum = 3
)

func checkErr(action string, err error){
    if err!=nil {
        log.Panicf("%s caused an error: %v", action, err)
    }
}

func main(){
    kingpin.MustParse(app.Parse(os.Args[1:]))
    cAddr, err := utils.InitSCIONConnection(*clientAddress)
    checkErr("Init SCION", err)

    if cAddr.L4Port != 0{
        log.Panicf("Application port must be set to 0, currently its %d", cAddr.L4Port)
    }

    servers, err := utils.LoadServersConfigurationList(*serversFile)
    checkErr("Loading server file", err)

    chain, err := utils.LoadChain(*chainFile)
    checkErr("Loading chain file", err)

    quorum := defaultServerQuorum
    if quorum > len(servers) {
        log.Printf("Quorum set to %d servers because not enough valid servers were found to meet the default (%d)!\n", len(servers), quorum)
        quorum = len(servers)
    }

    var client lib.Client
    result, err := client.EstablishTime(chain, quorum, servers, cAddr)
    checkErr("Establishing time", err)

    for serverName, err := range result.ServerErrors {
        log.Printf("Failed to query server %q: %s", serverName, err)
    }

    if result.MonoUTCDelta == nil {
        log.Printf("Failed to get %d servers to agree on the time.\n", quorum)
    } else {
        nowUTC := time.Unix(0, int64(monotime.Now()+*result.MonoUTCDelta))
        nowRealTime := time.Now()

        fmt.Printf("Real-time delta: %s\n", nowRealTime.Sub(nowUTC))
        fmt.Printf("Obtained midpoint time is: %s \n", time.Unix(0, result.Midpoint.Int64()))
    }

    err = utils.SaveChain(*chainFile, chain, *maxChainSize)
    checkErr("Saving chain file", err)
}