package main

import (
    "log"
    "fmt"
    "os"
    "io"
    "sync"

    "gopkg.in/alecthomas/kingpin.v2"

    "github.com/netsec-ethz/scion-apps/netcat/utils"
)

var (
    // Connection
    SERVER_ADDRESS = kingpin.Arg("host-address", "Server SCION address (without the port)").Required().String()
    PORT = kingpin.Arg("port", "The host's port").Required().Uint16()
	USE_IA_SCIOND_PATH = kingpin.Flag("sciond-path-from-ia", "Use IA when resolving SCIOND socket path").Bool()
    SEND_PIPER_BYTE = kingpin.Flag("send-extra-byte", "Sends an extra byte when opening the connection").Default("false").Bool()
)

func main() {
    kingpin.Parse()

    // Initialize SCION library
    err := utils.InitSCION("", "", *USE_IA_SCIOND_PATH)
    if err != nil {
        log.Panicf("Error initializing SCION connection: %v", err)
    }

    conn, err := utils.DialSCION(fmt.Sprintf("%s:%v", *SERVER_ADDRESS, *PORT))
    if err != nil {
        log.Panicf("Error dialing remote: %v", err)
    }

    log.Printf("Connected!")

    if *SEND_PIPER_BYTE {
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


