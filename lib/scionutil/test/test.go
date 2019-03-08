package main

import (
	"fmt"
	"log"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	slog "github.com/scionproto/scion/go/lib/log"
)

func main() {
	// discard SCION logging
	slog.Root().SetHandler(slog.DiscardHandler())
	scionutil.InitSCIONString("17-ffaa:1:c2,[127.0.0.1]:0")

	ia, host, err := scionutil.GetHostByName("umbrail.node.snet.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("umbrail.node.snet.: %s,[%s]\n", ia, host)

	ia, host, err = scionutil.GetHostByName("minimal-server")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("minimal-server: %s,[%s]\n", ia, host)
}
