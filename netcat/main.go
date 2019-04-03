package main

import (
	"flag"
	"fmt"
	"io"
	golog "log"
	"os"
	"strconv"
	"sync"

	quic "github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]:42002")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -local: Local SCION address (default localhost)")
	fmt.Println("  -b: Send an extra byte before sending the actual data")
}

func main() {
	log.SetupLogConsole("debug")
	log.Debug("Launching netcat")

	var (
		remoteAddressString string
		port                uint16
		localAddrString     string
		extraByte           bool
	)
	flag.Usage = printUsage
	flag.StringVar(&remoteAddressString, "local", "", "Local address string")
	flag.BoolVar(&extraByte, "b", false, "Send extra byte")
	flag.Parse()

	tail := flag.Args()
	if len(tail) != 2 {
		printUsage()
		golog.Panicf("Number of arguments is not two! Arguments: %v", tail)
	}

	remoteAddressString = tail[0]
	port64, err := strconv.ParseUint(tail[1], 10, 16)
	if err != nil {
		printUsage()
		golog.Panicf("Can't parse port string %v: %v", port64, err)
	}
	port = uint16(port64)

	if localAddrString == "" {
		localAddrString, err = scionutil.GetLocalhostString()
		if err != nil {
			golog.Panicf("Error getting localhost: %v", err)
		}
	}

	localAddr, err := snet.AddrFromString(localAddrString)
	if err != nil {
		golog.Panicf("Error parsing local address: %v", err)
	}

	// Initialize SCION library
	err = scionutil.InitSCION(localAddr)
	if err != nil {
		golog.Panicf("Error initializing SCION connection: %v", err)
	}

	remoteAddr, err := snet.AddrFromString(fmt.Sprintf("%s:%v", remoteAddressString, port))
	if err != nil {
		golog.Panicf("Can't parse remote address %s: %v", remoteAddressString)
	}

	sess, err := squic.DialSCION(nil, localAddr, remoteAddr, &quic.Config{KeepAlive: true})
	if err != nil {
		golog.Panicf("Can't dial remote address %s: %v", remoteAddressString, err)
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		golog.Panicf("Can't open stream: %v", err)
	}

	log.Debug("Connected!")

	if extraByte {
		_, err := stream.Write([]byte{71})
		if err != nil {
			golog.Panicf("Error writing extra byte: %v", err)
		}

		log.Debug("Sent extra byte!")
	}

	close := func() {
		err := stream.Close()
		if err != nil {
			log.Warn("Error closing stream: %v", err)
		}
		err = sess.Close(nil)
		if err != nil {
			log.Warn("Error closing session: %v", err)
		}
	}

	var once sync.Once
	go func() {
		io.Copy(os.Stdout, stream)
		once.Do(close)
	}()
	io.Copy(stream, os.Stdin)
	once.Do(close)

	log.Debug("Exiting snetcat...")
}
