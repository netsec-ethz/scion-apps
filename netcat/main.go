package main

import (
	"flag"
	"fmt"
	"io"
	golog "log"
	"os"
	"strconv"
	"sync"

	"github.com/netsec-ethz/scion-apps/netcat/utils"
	"github.com/scionproto/scion/go/lib/log"
)

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]:42002")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -P: Use IA when resolving SCIOND socket path")
	fmt.Println("  -b: Send an extra byte before sending the actual data")
}

func main() {
	log.SetupLogConsole("debug")
	log.Debug("Launching netcat")

	var (
		serverAddress   string
		port            uint16
		useIASCIONDPath bool
		extraByte       bool
	)
	flag.BoolVar(&showHelp, "h", false, "Show help")
	flag.BoolVar(&USE_IA_SCIOND_PATH, "P", false, "Use IA SCIOND Path")
	flag.BoolVar(&SEND_PIPER_BYTE, "b", false, "Send extra byte")
	flag.Parse()
	if showHelp {
		printUsage()
		return
	}

	tail := flag.Args()
	if len(tail) != 2 {
		golog.Panicf("Number of arguments is not two! Arguments: %v", tail)
	}

	SERVER_ADDRESS = tail[0]
	port64, err := strconv.ParseUint(tail[1], 10, 16)
	if err != nil {
		golog.Panicf("Can't parse port string %v: %v", port64, err)
	}
	PORT = uint16(port64)

	// Initialize SCION library
	err = utils.InitSCION("", "", USE_IA_SCIOND_PATH)
	if err != nil {
		golog.Panicf("Error initializing SCION connection: %v", err)
	}

	conn, err := utils.DialSCION(fmt.Sprintf("%s:%v", SERVER_ADDRESS, PORT))
	if err != nil {
		golog.Panicf("Error dialing remote: %v", err)
	}

	log.Debug("Connected!")

	if SEND_PIPER_BYTE {
		_, err := conn.Write([]byte{71})
		if err != nil {
			golog.Panicf("Error writing extra byte: %v", err)
		}

		log.Debug("Sent extra byte!")
	}

	close := func() {
		conn.Close()
	}

	var once sync.Once
	go func() {
		io.Copy(os.Stdout, conn)
		once.Do(close)
	}()
	io.Copy(conn, os.Stdin)
	once.Do(close)

	log.Debug("Exiting snetcat...")
}
