// sensorserver application
// For documentation on how to setup and run the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

const (
	TIMESTRING             string = "Time"
	TIMEFORMAT             string = "2006/01/02 15:04:05"
	SEPARATORSTRING        string = ": "
	TIMEANDSEPARATORSTRING string = TIMESTRING + SEPARATORSTRING
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var sensorData map[string]string
var sensorDataLock sync.Mutex

func init() {
	sensorData = make(map[string]string)
}

// Obtains input from sensor observation application
func parseInput() {
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()
		index := strings.Index(line, TIMEANDSEPARATORSTRING)
		if index == 0 {
			// We found a time string, format in case parsing is desired: 2017/11/16 21:29:49
			timestr := line[len(TIMEANDSEPARATORSTRING):]
			sensorDataLock.Lock()
			sensorData[TIMESTRING] = timestr
			sensorDataLock.Unlock()
			continue
		}
		index = strings.Index(line, SEPARATORSTRING)
		if index > 0 {
			sensorType := line[:index]
			sensorDataLock.Lock()
			sensorData[sensorType] = line
			sensorDataLock.Unlock()
		}
	}
}

func printUsage() {
	fmt.Println("sensorserver -s ServerSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 17-ffaa:0:1102,[192.33.93.173]:42002")
}

func main() {
	go parseInput()

	var (
		serverAddress  string
		serverPort     uint
		sciondPath     string
		sciondFromIA   bool
		dispatcherPath string

		err    error
		server *snet.Addr

		udpConnection snet.Conn
	)

	// Fetch arguments from command line
	flag.StringVar(&serverAddress, "s", "", "Server SCION Address")
	flag.UintVar(&serverPort, "p", 40002, "Server Port (only used when Server Address not set)")
	flag.StringVar(&sciondPath, "sciond", "", "Path to sciond socket")
	flag.BoolVar(&sciondFromIA, "sciondFromIA", false, "SCIOND socket path from IA address:ISD-AS")
	flag.StringVar(&dispatcherPath, "dispatcher", "/run/shm/dispatcher/default.sock",
		"Path to dispatcher socket")
	flag.Parse()

	var pflag bool
	var sflag bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "s" {
			sflag = true
		}
		if f.Name == "p" {
			pflag = true
		}
	})
	if sflag && pflag {
		log.Println("Warning: flags '-s' and '-p' provided. '-p' has no effect")
	}

	// Create the SCION UDP socket
	if len(serverAddress) > 0 {
		server, err = snet.AddrFromString(serverAddress)
		check(err)
		if server.Host.L4 == nil {
			log.Fatal("Port in server address is missing")
		}
	} else {
		server, err = scionutil.GetLocalhost()
		check(err)
		server.Host.L4 = addr.NewL4UDPInfo(uint16(serverPort))
	}

	if sciondFromIA {
		if sciondPath != "" {
			log.Fatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		sciondPath = sciond.GetDefaultSCIONDPath(&server.IA)
	} else if sciondPath == "" {
		sciondPath = sciond.GetDefaultSCIONDPath(nil)
	}
	snet.Init(server.IA, sciondPath, reliable.NewDispatcherService(dispatcherPath))
	udpConnection, err = snet.ListenSCION("udp4", server)
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		_, clientAddress, err := udpConnection.ReadFrom(receivePacketBuffer)
		check(err)

		// Packet received, send back response to same client
		var sensorValues string
		var timeString string
		sensorDataLock.Lock()
		for k, v := range sensorData {
			if strings.Index(k, TIMESTRING) == 0 {
				timeString = v
			} else {
				sensorValues = sensorValues + v + "\n"
			}
		}
		sensorDataLock.Unlock()
		sensorValues = timeString + "\n" + sensorValues
		copy(sendPacketBuffer, sensorValues)

		_, err = udpConnection.WriteTo(sendPacketBuffer[:len(sensorValues)], clientAddress)
		check(err)
	}
}
