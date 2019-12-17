// sensorfetcher application
// For documentation on how to setup and run the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func main() {

	serverAddrStr := flag.String("s", "", "Server address (<ISD-AS,[IP]:port> or <hostname:port>)")
	flag.Parse()

	if len(*serverAddrStr) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	conn, err := scionutil.Dial(*serverAddrStr)
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 0)

	_, err = conn.Write(sendPacketBuffer)
	check(err)

	n, err := conn.Read(receivePacketBuffer)
	check(err)

	fmt.Print(string(receivePacketBuffer[:n]))
}
