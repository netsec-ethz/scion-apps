package main

import (
	"log"

	"github.com/scionproto/scion/go/lib/snet"
	quic "github.com/scionproto/scion/go/lib/snet/squic"
	"github.com/xabarass/scion-http/utils"
)

func initSCIONConnection(serverAddress, tlsCertFile, tlsKeyFile string) {
	log.Printf("Initializing SCION connection")

	// Initialize default SCION networking context
	if err := snet.Init(serverAddress.IA, utils.GetScionAddr(serverAddress), utils.GetDispatcherAddr(serverAddress)); err != nil {
		log.Printf("Unable to initialize SCION network: %v", err)
		return
	}

	// Initialize the QUIC connection
	if err = quic.Init(tlsKeyFile, tlsCertFile); err != nil {
		log.Printf("Failed to set up QUIC connection: %v", err)
		return
	}

	log.Printf("SCION/QUIC successfully initialized")
}

func main() {

}
