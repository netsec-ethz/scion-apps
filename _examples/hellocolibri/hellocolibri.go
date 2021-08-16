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
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	libcol "github.com/scionproto/scion/go/lib/colibri"
	colapi "github.com/scionproto/scion/go/lib/colibri/client"
	"github.com/scionproto/scion/go/lib/colibri/reservation"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sciond"
	colpath "github.com/scionproto/scion/go/lib/slayers/path/colibri"
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
	// client(serverChan)
	fmt.Println("==============================================================================================")
	// after client is finished, run the extended API example
	clientExtendedAPI(serverChan)
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

	var stitchable *libcol.StitchableSegments
	for {
		stitchable, err = daemon.ColibriListRsvs(ctx, dstIA)
		check(err)
		if stitchable != nil {
			break
		}
		time.Sleep(time.Second)
	}
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
			Suffix: make([]byte, 12),
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

	setupReq.RequestedBW = 9
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
		RequestedBW: 9,
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
	udpAddr.Port = 44321 // TODO(juagargi) or zero?

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
	cp := &colpath.ColibriPath{}
	err = cp.DecodeFromBytes(colibriPath.Raw)
	check(err)
	fmt.Printf("We will send traffic using colibri now, with path %s\n", printPath(cp))
	serverAddr.Path.Type = colpath.PathType
	serverAddr.Path.Raw = colibriPath.Raw

	conn, err = scionNet.Dial(ctx, "udp", udpAddr, serverAddr, addr.SvcNone)
	check(err)
	_, err = conn.Write([]byte("hello there, colibri carried data"))
	check(err)
	data = <-serverChan
	fmt.Printf("we know the server got data: %s\n", string(data))
	conn.Close()

	// clean the last reservation obtained
	err = daemon.ColibriCleanupRsv(ctx, &setupReq2.Id, setupReq2.Index)
	check(err)
	fmt.Println("Reservation cleaned up")
}

func clientExtendedAPI(serverChan chan []byte) {
	fmt.Println("client started")
	ctx, cancelF := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelF()

	localUdpAddr, err := net.ResolveUDPAddr("udp", clientSciondPath)
	check(err)
	localUdpAddr.Port = 44321 // TODO(juagargi) or zero?
	dstUdpAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	dstUdpAddr.Port = serverPort
	dstIA, err := addr.IAFromString("1-ff00:0:112")
	check(err)

	daemon, err := sciond.NewService(clientSciondPath).Connect(ctx)
	check(err)
	localIA, err := daemon.LocalIA(ctx)
	check(err)
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, sciond.RevHandler{Connector: daemon})
	dstAddr := &snet.UDPAddr{
		IA:   dstIA,
		Host: dstUdpAddr,
	}

	fmt.Println("deleteme 1")
	rsv, err := colapi.NewReservation(ctx, scionNet, daemon, dstAddr, 9, 0, func(a, b libcol.FullTrip) bool {
		// sorting function, return true if a < b. The first trip will be the chosen one
		return a.ExpirationTime().Before(b.ExpirationTime())
	})
	check(err)
	fmt.Println("deleteme 2")

	// auto renew reservation
	err = rsv.Open(ctx, localUdpAddr, func(r *colapi.Reservation, err error) {
		fmt.Printf("aieeee, failed to renew, error type: %s, message: %s\n",
			common.TypeOf(err), err)
		panic("")
	})
	check(err)
	fmt.Println("deleteme 3")

	defer func() {
		fmt.Printf("closing reservation... ")
		err = rsv.Close(ctx)
		check(err)
		fmt.Println("done.")
	}()

	t1 := time.Now()
	_, err = rsv.Write([]byte("hello there, colibri carried data with extended API"))
	check(err)
	fmt.Println("deleteme 4")
	data := <-serverChan
	fmt.Printf("server got data: %s\n", string(data))

	elapsed := time.Since(t1)
	if sleepDur := reservation.E2ERsvDuration - elapsed + time.Second; sleepDur > 0 {
		fmt.Printf("Sleeping until current e2e reservation expires >=1 times: %s\n", sleepDur)
		time.Sleep(sleepDur)
		fmt.Println("Awaken now")
	}
	fmt.Println("sending a second message to the server")
	_, err = rsv.Write([]byte("hello again, probably using the first renewed reservation"))
	check(err)
	data = <-serverChan
	fmt.Printf("server got data: %s\n", string(data))
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
func printPath(p *colpath.ColibriPath) string {
	hfs := make([]string, len(p.HopFields))
	for i, hf := range p.HopFields {
		hfs[i] = fmt.Sprintf("[%d %d]", hf.IngressId, hf.EgressId)
	}
	return fmt.Sprintf("TS:%d, exp: %s, hops: %s",
		p.PacketTimestamp,
		reservation.Tick(p.InfoField.ExpTick).ToTime(),
		strings.Join(hfs, " -> "))
}
