package main

import (
    "log"
    "fmt"
    "os"
    "io"
    "sync"
    "strconv"
    "flag"

    "github.com/netsec-ethz/scion-apps/netcat/utils"
)

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]:42002")
	fmt.Println("Available flags:")
	fmt.Println("  --sciond-path-from-ia: Use IA when resolving SCIOND socket path")
	fmt.Println("  --send-extra-byte: Send an extra byte before sending the actual data")
}

func main() {
    var (
        SERVER_ADDRESS string
        PORT uint16
        USE_IA_SCIOND_PATH bool
        SEND_PIPER_BYTE bool
    )

    flag.BoolVar(&USE_IA_SCIOND_PATH, "d", false, "Use IA SCIOND Path")
    flag.BoolVar(&SEND_PIPER_BYTE, "b", false, "Send extra byte")
    flag.Parse()
    tail := flag.Args()
    if len(tail) != 2 {
        log.Panicf("Number of arguments is not two! Arguments: %v", tail)
    }

    SERVER_ADDRESS = tail[0]
    port64, err := strconv.ParseUint(tail[1], 10, 16)
    if err != nil {
        log.Panicf("Can't parse port string %s: %v", port64, err)
    }
    PORT = uint16(port64)


    // Initialize SCION library
    err = utils.InitSCION("", "", USE_IA_SCIOND_PATH)
    if err != nil {
        log.Panicf("Error initializing SCION connection: %v", err)
    }

    conn, err := utils.DialSCION(fmt.Sprintf("%s:%v", SERVER_ADDRESS, PORT))
    if err != nil {
        log.Panicf("Error dialing remote: %v", err)
    }

    log.Printf("Connected!")

    if SEND_PIPER_BYTE {
        _, err := conn.Write([]byte {71})
        if err != nil {
            log.Panicf("Error writing extra byte: %v", err)
        }

        log.Printf("Sent extra byte!")
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

    log.Printf("Exiting snetcat...")
}


