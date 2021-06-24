// Copyright 2021 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	libcol "github.com/scionproto/scion/go/lib/colibri"
	"github.com/scionproto/scion/go/lib/colibri/reservation"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/slayers/path/colibri"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

const (
	serverSciondPath = "[fd00:f00d:cafe::7f00:c]:30255" // AS 1-ff00:0:112
	clientSciondPath = "127.0.0.20:30255"               // AS 1-ff00:0:111
	serverPort       = 12345
)

func main() {
	serverChan := make(chan []byte)
	server(serverChan)
	client(serverChan)
}

func server(serverChan chan []byte) {
	fmt.Println("server starting")
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	daemon, err := sciond.NewService(serverSciondPath).Connect(ctx)
	check(err)

	localIA, err := daemon.LocalIA(ctx)
	check(err)
	udpAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	udpAddr.Port = serverPort

	// init network
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, sciond.RevHandler{Connector: daemon})

	fmt.Printf("server at %s\n", udpAddr)
	conn, err := scionNet.Listen(context.Background(), "udp", udpAddr, addr.SvcNone)
	check(err)

	// serve detached
	go func() {
		for {
			buffer := make([]byte, 16384)
			n, from, err := conn.ReadFrom(buffer)
			check(err)
			data := buffer[:n]
			fmt.Printf("server got %d bytes from %s: %s\n", n, from, data)
			serverChan <- data
		}
	}()
}

func client(serverChan chan []byte) {
	fmt.Println("client started")
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	daemon, err := sciond.NewService(clientSciondPath).Connect(ctx)
	check(err)

	localIA, err := daemon.LocalIA(ctx)
	check(err)

	// init network for later use
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, sciond.RevHandler{Connector: daemon})

	dstIA, err := addr.IAFromString("1-ff00:0:112")
	check(err)

	stitchable, err := daemon.ColibriListRsvs(ctx, dstIA)
	check(err)
	fmt.Printf("received reservations to %s:\n%+v\n", dstIA, stitchable)

	trips := libcol.CombineAll(stitchable)
	fmt.Printf("Got %d trips\n", len(trips))
	for i, t := range trips {
		fmt.Printf("[%3d]: %s\n", i, t)
	}
	if len(trips) == 0 {
		check(fmt.Errorf("no trips"))
	}

	// reservation requests
	fmt.Printf("\nThis first reservation should fail\n")

	setupReq := &libcol.E2EReservationSetup{
		Id: reservation.ID{
			ASID:   localIA.A,
			Suffix: make([]byte, 10),
		},
		SrcIA:       localIA,
		DstIA:       dstIA,
		Index:       0, // new index
		Segments:    trips[0].Segments(),
		RequestedBW: 11,
	}
	rand.Read(setupReq.Id.Suffix) // random suffix
	_, err = daemon.ColibriSetupRsv(ctx, setupReq)
	if err == nil {
		check(fmt.Errorf("expected error but got nil"))
	}
	fmt.Printf("expected error in admission: %s\n", err)
	if e2eerr, ok := err.(*libcol.E2ESetupError); ok {
		fmt.Printf("admission error: failed at AS %d, trail: %v\n",
			e2eerr.FailedAS, e2eerr.AllocationTrail)
	}

	setupReq.RequestedBW = 10
	fmt.Printf("\nGoing again with requested BW cls = %d, ID: %s\n",
		setupReq.RequestedBW, setupReq.Id)
	res, err := daemon.ColibriSetupRsv(ctx, setupReq)
	check(err)
	fmt.Println("admitted")
	fmt.Printf("path type: %s\n", res.Path().Type)

	// try to get again a new reservation. It should fail as there is no more bandwidth.
	setupReq2 := &libcol.E2EReservationSetup{
		Id:          *setupReq.Id.Copy(),
		SrcIA:       setupReq.SrcIA,
		DstIA:       setupReq.DstIA,
		Index:       0,
		Segments:    setupReq.Segments,
		RequestedBW: 10,
	}
	rand.Read(setupReq2.Id.Suffix) // new reservation
	fmt.Printf("\nRequesting a new reservation with same BW, new id: %s\n", setupReq2.Id)
	_, err = daemon.ColibriSetupRsv(ctx, setupReq2)
	if err == nil {
		check(fmt.Errorf("expected error but got nil"))
	}
	e2eerr, ok := err.(*libcol.E2ESetupError)
	if !ok {
		check(fmt.Errorf("expected error type E2ESetupError, but got %T", err))
	}
	fmt.Printf("expected admission error: failed at AS %d, trail: %v\n",
		e2eerr.FailedAS, e2eerr.AllocationTrail)

	// cleanup the first one and attempt again
	fmt.Printf("\nWill clean first rsv, id: %s\n", setupReq.Id)
	err = daemon.ColibriCleanupRsv(ctx, &setupReq.Id, setupReq.Index)
	check(err)
	fmt.Printf("Cleaned first reservation\n")
	// go again, this time it should succeed
	fmt.Printf("Requesting a new reservation with same BW, id: %s\n", setupReq2.Id)
	res, err = daemon.ColibriSetupRsv(ctx, setupReq2)
	check(err)
	fmt.Println("admitted")
	fmt.Printf("path type: %s\n", res.Path().Type)
	colibriPath := res.Path()

	//////////////////////////////////////////////////////////////////////////////////
	//
	// find the server address
	pathquerier := sciond.Querier{
		Connector: daemon,
		IA:        localIA,
	}
	pathsToDst, err := pathquerier.Query(ctx, dstIA)
	check(err)
	p := pathsToDst[0]
	fmt.Printf("Will connect to server using path %s\n", p)

	serverUdpAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	serverUdpAddr.Port = serverPort
	serverAddr := &snet.UDPAddr{
		IA:      dstIA,
		Path:    p.Path(),
		Host:    serverUdpAddr,
		NextHop: p.UnderlayNextHop(),
	}
	// connect to the server
	udpAddr, err := net.ResolveUDPAddr("udp", clientSciondPath)
	check(err)
	udpAddr.Port = 44321 // or zero

	// connect to server
	conn, err := scionNet.Dial(ctx, "udp", udpAddr, serverAddr, addr.SvcNone)
	check(err)
	_, err = conn.Write([]byte("hello there, best effort"))
	check(err)
	data := <-serverChan
	fmt.Printf("we know the server got data: %s\n", string(data))
	conn.Close()
	//
	//
	// now dial using colibri
	fmt.Println("We will send traffic using colibri now")
	serverAddr.Path.Type = colibri.PathType
	serverAddr.Path.Raw = colibriPath.Raw

	conn, err = scionNet.Dial(ctx, "udp", udpAddr, serverAddr, addr.SvcNone)
	check(err)
	_, err = conn.Write([]byte("hello there, best effort"))
	check(err)
	data = <-serverChan
	fmt.Printf("we know the server got data: %s\n", string(data))
	conn.Close()

	// clean the last reservation obtained
	err = daemon.ColibriCleanupRsv(ctx, &setupReq2.Id, setupReq2.Index)
	check(err)
	fmt.Println("Reservation cleaned up")
}

// check just ensures the error is nil, or complains and quits
func check(err error) {
	if err != nil {
		fmt.Println("\n--- PANIC ---")
		switch e2eerr := err.(type) {
		case *libcol.E2ESetupError:
			fmt.Printf("%s\nAllocTrail: %v\n", err, e2eerr.AllocationTrail)
		case *libcol.E2EResponseError:
		}
		panic(fmt.Sprintf("%s Fatal error: %s", time.Now(), err))
	}
}
