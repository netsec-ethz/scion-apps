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
	"github.com/scionproto/scion/go/lib/colibri"
	libcol "github.com/scionproto/scion/go/lib/colibri"
	colapi "github.com/scionproto/scion/go/lib/colibri/client"
	"github.com/scionproto/scion/go/lib/colibri/client/fallingback"
	"github.com/scionproto/scion/go/lib/colibri/client/sorting"
	"github.com/scionproto/scion/go/lib/colibri/reservation"
	"github.com/scionproto/scion/go/lib/daemon"
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
	fmt.Println("=================================================================================")
	client(serverChan)
	fmt.Println("=================================================================================")
	// after client is finished, run the extended API example
	clientExtendedAPI(serverChan)
}

func server(serverChan chan []byte) {
	fmt.Println("server starting")
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	connector, err := daemon.NewService(serverSciondPath).Connect(ctx)
	check(err)

	localIA, err := connector.LocalIA(ctx)
	check(err)
	udpAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	udpAddr.Port = serverPort

	// init network
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, daemon.RevHandler{Connector: connector})

	fmt.Printf("server at %s\n", udpAddr)
	conn, err := scionNet.Listen(context.Background(), "udp", udpAddr, addr.SvcNone)
	check(err)

	// periodically set ourselves as ready to receive reservations (whitelist all)
	go func() {
		for {
			ctx, cancelF := context.WithTimeout(context.Background(), 3*time.Second)
			entry := &colibri.AdmissionEntry{
				DstHost:         udpAddr.IP, // could be empty to detect it automatically
				ValidUntil:      time.Now().Add(time.Minute),
				RegexpIA:        "", // from any AS
				RegexpHost:      "", // from any host
				AcceptAdmission: true,
			}
			fmt.Printf("server, adding admission entry for %s\n", udpAddr.IP)
			validUntil, err := connector.ColibriAddAdmissionEntry(ctx, entry)
			check(err)
			if time.Until(validUntil).Seconds() < 45 {
				check(fmt.Errorf("too short validity, something went wrong. "+
					"Requested %s, got %s", entry.ValidUntil, validUntil))
			}
			cancelF()
			time.Sleep(30 * time.Second)
		}
	}()

	// serve detached
	go func() {
		for {
			buffer := make([]byte, 16384)
			n, from, err := conn.ReadFrom(buffer)
			check(err)
			data := buffer[:n]
			fromScion := from.(*snet.UDPAddr)
			fmt.Printf("server got %d bytes from %s (path type%s). Message is: %s\n",
				n, from, &fromScion.Path.Type, data)
			serverChan <- data
		}
	}()
}

func client(serverChan chan []byte) {
	fmt.Println("client started")
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	connector, err := daemon.NewService(clientSciondPath).Connect(ctx)
	check(err)

	localIA, err := connector.LocalIA(ctx)
	check(err)

	// init network for later use
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, daemon.RevHandler{Connector: connector})

	dstIA, err := addr.IAFromString("1-ff00:0:112")
	check(err)

	var stitchable *libcol.StitchableSegments
	for {
		stitchable, err = connector.ColibriListRsvs(ctx, dstIA)
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

	serverUDPAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	setupReq := &libcol.E2EReservationSetup{
		Id: reservation.ID{
			ASID:   localIA.A,
			Suffix: make([]byte, 12),
		},
		SrcIA:       localIA,
		DstIA:       dstIA,
		DstHost:     serverUDPAddr.IP,
		Index:       0, // new index
		Segments:    trips[0].Segments(),
		RequestedBW: 13,
	}
	rand.Read(setupReq.Id.Suffix) // random suffix
	_, err = connector.ColibriSetupRsv(ctx, setupReq)
	if err == nil {
		check(fmt.Errorf("expected error but got nil"))
	}
	fmt.Printf("expected error in admission: %s\n", err)
	admissionFailure, ok := err.(*libcol.E2ESetupError)
	if !ok {
		check(fmt.Errorf("expected error type E2ESetupError, but got %T", err))
	}
	fmt.Printf("admission error: failed at AS %d, trail: %v\n",
		admissionFailure.FailedAS, admissionFailure.AllocationTrail)

	setupReq.RequestedBW = 11
	fmt.Printf("\nGoing again with requested BW cls = %d, ID: %s\n",
		setupReq.RequestedBW, setupReq.Id)
	res, err := connector.ColibriSetupRsv(ctx, setupReq)
	check(err)
	fmt.Println("admitted")
	fmt.Printf("path type: %s\n", res.Path().Type)

	// try to get again a new reservation. It should fail as there is no more bandwidth.
	setupReq2 := &libcol.E2EReservationSetup{
		Id:          *setupReq.Id.Copy(),
		SrcIA:       setupReq.SrcIA,
		DstIA:       setupReq.DstIA,
		DstHost:     serverUDPAddr.IP,
		Index:       0,
		Segments:    setupReq.Segments,
		RequestedBW: 11,
	}
	rand.Read(setupReq2.Id.Suffix) // new reservation
	fmt.Printf("\nRequesting a new reservation with same BW, new id: %s\n", setupReq2.Id)
	_, err = connector.ColibriSetupRsv(ctx, setupReq2)
	if err == nil {
		check(fmt.Errorf("expected error but got nil"))
	}
	admissionFailure, ok = err.(*libcol.E2ESetupError)
	if !ok {
		check(fmt.Errorf("expected error type E2ESetupError, but got %T", err))
	}
	fmt.Printf("expected admission error: failed at AS %d, trail: %v\n",
		admissionFailure.FailedAS, admissionFailure.AllocationTrail)

	// cleanup the first one and attempt again
	fmt.Printf("\nWill clean first rsv, id: %s\n", setupReq.Id)
	err = connector.ColibriCleanupRsv(ctx, &setupReq.Id, setupReq.Index)
	check(err)
	fmt.Printf("Cleaned first reservation\n")
	// go again, this time it should succeed
	fmt.Printf("Requesting a new reservation with same BW, id: %s\n", setupReq2.Id)
	res, err = connector.ColibriSetupRsv(ctx, setupReq2)
	check(err)
	fmt.Println("admitted")
	fmt.Printf("path type: %s\n", res.Path().Type)
	colibriPath := res.Path()

	//////////////////////////////////////////////////////////////////////////////////
	//
	// find the server address
	pathquerier := daemon.Querier{
		Connector: connector,
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
	err = connector.ColibriCleanupRsv(ctx, &setupReq2.Id, setupReq2.Index)
	check(err)
	fmt.Println("Reservation cleaned up")
}

// clientExtendedAPI shows an example of an upgrade of a regular SCION connection to a
// COLIBRI one, and the use of the COLIBRI extended API.
func clientExtendedAPI(serverChan chan []byte) {
	fmt.Println("client started")
	ctx, cancelF := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelF()

	// setup the scenario with data from the local topology tiny.topo:
	localUdpAddr, err := net.ResolveUDPAddr("udp", clientSciondPath)
	check(err)
	localUdpAddr.Port = 44321 // TODO(juagargi) or zero?
	dstUdpAddr, err := net.ResolveUDPAddr("udp", serverSciondPath)
	check(err)
	dstUdpAddr.Port = serverPort
	dstIA, err := addr.IAFromString("1-ff00:0:112")
	check(err)

	connector, err := daemon.NewService(clientSciondPath).Connect(ctx)
	check(err)
	localIA, err := connector.LocalIA(ctx)
	check(err)
	dispatcher := reliable.NewDispatcher(reliable.DefaultDispPath)
	scionNet := snet.NewNetwork(localIA, dispatcher, daemon.RevHandler{Connector: connector})
	dstAddr := &snet.UDPAddr{
		IA:   dstIA,
		Host: dstUdpAddr,
	}

	// While we have no reservation yet, let's create a regular connection to the server.
	// We will attempt to upgrade it to a colibri backed connection, later.
	conn, err := scionNet.Dial(ctx, "udp", localUdpAddr, dstAddr, addr.SvcNone)
	check(err)
	// here we could send/receive data using the connection conn, while we create a reservation.
	// ...

	// create a reservation
	capturedTrips := make([]*colibri.FullTrip, 0)
	rsv, err := colapi.NewReservation(ctx, connector, localIA, dstIA, dstAddr.Host.IP, 9, 0,
		// we record the trips, sort by BW and then sort by number of ASes:
		fallingback.CaptureTrips(&capturedTrips), sorting.ByBW, sorting.ByNumberOfASes,
	)
	check(err)

	// Auto renew reservation, fallback to next trip without the failing interface.
	// We could also provide our own fallback function instead.
	err = rsv.Open(ctx,
		func(r *colapi.Reservation) { // on renewal, copy the path to dstAddr
			// dstAddr.NextHop = r.UnderlayNextHop() // not strictly necessary, should be the same
			dstAddr.Path = r.Path()
		},
		fallingback.SkipInterface(capturedTrips))
	check(err)
	// do not forget to update the dstAddr with the data from the first setup:
	dstAddr.NextHop = rsv.UnderlayNextHop()
	dstAddr.Path = rsv.Path()
	// and do not forget to close the reservation
	defer func() {
		fmt.Printf("closing reservation... ")
		err = rsv.Close(ctx)
		check(err)
		fmt.Println("done.")
	}()

	// now, just use the connection `conn` normally
	t1 := time.Now()
	// use an existing connection to send colibri packets anytime (upgrade usage of `conn`)
	_, err = conn.WriteTo([]byte("hello there, colibri carried data with extended API. "+
		"If you don't see this message, you probably forgot to set the nexthop and path of"+
		"the destination address right after opening the Reservation."), dstAddr)
	check(err)
	data := <-serverChan
	fmt.Printf("server got data: %d bytes\n", len(data))

	elapsed := time.Since(t1)
	if sleepDur := reservation.E2ERsvDuration - elapsed + time.Second; sleepDur > 0 {
		fmt.Printf("Sleeping until current e2e reservation expires >=1 times: %s\n", sleepDur)
		time.Sleep(sleepDur)
		fmt.Println("Awaken now")
	}
	fmt.Println("sending a second message to the server")
	_, err = conn.WriteTo([]byte("hello again, probably using the first renewed reservation\n"+
		"If you don't see this message, you probably forgot to update the destination address "+
		"with the renewed colibri path, via the \"successFcn\" callback function in Open."),
		dstAddr)
	check(err)
	data = <-serverChan
	fmt.Printf("server got data: %d\n", len(data))
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
		panic(fmt.Sprintf("%s Fatal error: %s", time.Now().Format(time.StampMilli), err))
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
