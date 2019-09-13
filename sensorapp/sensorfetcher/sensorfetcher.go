// sensorfetcher application
// For documentation on how to setup and run the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func printUsage() {
	fmt.Println("scion-sensor-server -s ServerSCIONAddress -c ClientSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1,[127.0.0.1]:42002")
	fmt.Println("ClientSCIONAddress can be omitted, the application then binds to localhost")
}

func main() {
	var (
		clientAddress  string
		serverAddress  string
		sciondPath     string
		sciondFromIA   bool
		dispatcherPath string

		err    error
		local  *snet.Addr
		remote *snet.Addr

		udpConnection snet.Conn
	)

	// Fetch arguments from command line
	flag.StringVar(&clientAddress, "c", "", "Client SCION Address")
	flag.StringVar(&serverAddress, "s", "", "Server SCION Address")
	flag.StringVar(&sciondPath, "sciond", "", "Path to sciond socket")
	flag.BoolVar(&sciondFromIA, "sciondFromIA", false, "SCIOND socket path from IA address:ISD-AS")
	flag.StringVar(&dispatcherPath, "dispatcher", "/run/shm/dispatcher/default.sock",
		"Path to dispatcher socket")
	flag.Parse()

	// Create the SCION UDP socket
	if len(clientAddress) > 0 {
		local, err = snet.AddrFromString(clientAddress)
	} else {
		local, err = scionutil.GetLocalhost()
	}
	check(err)

	if len(serverAddress) > 0 {
		remote, err = snet.AddrFromString(serverAddress)
		check(err)
	} else {
		printUsage()
		check(fmt.Errorf("Error, server address needs to be specified with -s"))
	}

	if sciondFromIA {
		if sciondPath != "" {
			log.Fatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		sciondPath = sciond.GetDefaultSCIONDPath(&local.IA)
	} else if sciondPath == "" {
		sciondPath = sciond.GetDefaultSCIONDPath(nil)
	}
	snet.Init(local.IA, sciondPath, reliable.NewDispatcherService(dispatcherPath))
	udpConnection, err = snet.DialSCION("udp4", local, remote)
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 0)

	_, err = udpConnection.Write(sendPacketBuffer)
	check(err)

	n, _, err := udpConnection.ReadFrom(receivePacketBuffer)
	check(err)

	fmt.Print(string(receivePacketBuffer[:n]))
}
