// Copyright 2020 ETH Zurich
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
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/daemon"
	"github.com/scionproto/scion/go/lib/drkey"
	"github.com/scionproto/scion/go/lib/drkey/fetcher"
	"github.com/scionproto/scion/go/lib/scrypto/cppki"
	cppb "github.com/scionproto/scion/go/pkg/proto/control_plane"
	dkpb "github.com/scionproto/scion/go/pkg/proto/drkey"
)

// check just ensures the error is nil, or complains and quits
func check(e error) {
	if e != nil {
		panic(fmt.Sprintf("Fatal error: %v", e))
	}
}

type dialer struct {
	daemon daemon.Connector
}

func (d *dialer) Dial(ctx context.Context, _ net.Addr) (*grpc.ClientConn, error) {
	// Obtain CS address from scion daemon
	svcs, err := d.daemon.SVCInfo(ctx, nil)
	check(err)
	cs := svcs[addr.SvcCS]
	if len(cs) == 0 {
		panic("no CS svc address")
	}

	// Contact CS directly for SV
	return grpc.DialContext(ctx, cs, grpc.WithInsecure())
}

type Client struct {
	fetcher *fetcher.FromCS
}

func NewClient(ctx context.Context, sciondPath string) Client {
	daemon, err := daemon.NewService(sciondPath).Connect(ctx)
	check(err)
	return Client{
		fetcher: &fetcher.FromCS{
			Dialer: &dialer{
				daemon: daemon,
			},
		},
	}
}

func (c Client) HostHostKey(ctx context.Context, meta drkey.HostHostMeta) drkey.HostHostKey {
	// get L2 key: (slow path)
	key, err := c.fetcher.DRKeyGetHostHostKey(ctx, meta)
	check(err)
	return key
}

type Server struct {
	daemon daemon.Connector
}

func NewServer(ctx context.Context, sciondPath string) Server {
	daemon, err := daemon.NewService(sciondPath).Connect(ctx)
	check(err)
	return Server{
		daemon: daemon,
	}
}

// fetchSV obtains the Secret Value (SV) for the selected protocol/epoch.
// From this SV, all keys for this protocol/epoch can be derived locally.
// The IP address of the server must be explicitly allowed to abtain this SV
// from the the control server.
func (s Server) fetchSV(ctx context.Context, meta drkey.SVMeta) drkey.SV {
	// Obtain CS address from scion daemon. Note there's no need to use
	// the daemon as long as a valid address could be passed to the dialing
	// function.
	svcs, err := s.daemon.SVCInfo(ctx, nil)
	check(err)
	cs := svcs[addr.SvcCS]
	if len(cs) == 0 {
		panic("no CS svc address")
	}

	// Contact CS directly for SV
	conn, err := grpc.DialContext(ctx, cs, grpc.WithInsecure())
	check(err)
	defer conn.Close()
	client := cppb.NewDRKeyIntraServiceClient(conn)

	rep, err := client.SV(ctx, &dkpb.SVRequest{
		ValTime:    timestamppb.New(meta.Validity),
		ProtocolId: dkpb.Protocol(meta.ProtoId),
	})
	check(err)
	key, err := getSecretFromReply(meta.ProtoId, rep)
	check(err)
	return key
}

func (s Server) HostHostKey(sv drkey.SV, meta drkey.HostHostMeta) drkey.HostHostKey {
	deriver := (&drkey.SpecificDeriver{})
	lvl1, err := deriver.DeriveLvl1(drkey.Lvl1Meta{
		Validity: meta.Validity,
		ProtoId:  meta.ProtoId,
		SrcIA:    meta.SrcIA,
		DstIA:    meta.DstIA,
	}, sv.Key)
	check(err)
	asHost, err := deriver.DeriveHostAS(drkey.HostASMeta{
		Lvl2Meta: meta.Lvl2Meta,
		SrcHost:  meta.SrcHost,
	}, lvl1)
	check(err)
	hosthost, err := deriver.DeriveHostToHost(meta.DstHost, asHost)
	check(err)
	return drkey.HostHostKey{
		ProtoId: sv.ProtoId,
		Epoch:   sv.Epoch,
		SrcIA:   meta.SrcIA,
		DstIA:   meta.DstIA,
		SrcHost: meta.SrcHost,
		DstHost: meta.DstHost,
		Key:     hosthost,
	}
}

func main() {
	const sciondForServer = "127.0.0.20:30255"
	serverIA, err := addr.ParseIA("1-ff00:0:111")
	check(err)
	const serverIP = "127.0.0.1"

	const sciondForClient = "[fd00:f00d:cafe::7f00:c]:30255"
	clientIA, err := addr.ParseIA("1-ff00:0:112")
	check(err)
	const clientIP = "fd00:f00d:cafe::7f00:b"

	ctx, cancelF := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelF()

	// meta describes the key that both client and server derive
	meta := drkey.HostHostMeta{
		Lvl2Meta: drkey.Lvl2Meta{
			ProtoId: drkey.SCMP,
			// Validity timestamp; both sides need to use the same time stamp when deriving the key
			// to ensure they derive keys for the same epoch.
			// Usually this is coordinated by means of a timestamp in the message.
			Validity: time.Now(),
			// SrcIA is the AS on the "fast side" of the DRKey derivation;
			// the server side in this example.
			SrcIA: serverIA,
			// DstIA is the AS on the "slow side" of the DRKey derivation;
			// the client side in this example.
			DstIA: clientIA,
		},
		SrcHost: serverIP,
		DstHost: clientIP,
	}

	// Client: fetch key from daemon
	// The daemon will in turn obtain the key from the local CS
	// The CS will fetch the Lvl1 key from the CS in the SrcIA (the server's AS)
	// and derive the Host key based on this.
	client := NewClient(ctx, sciondForClient)
	t0 := time.Now()
	clientKey := client.HostHostKey(ctx, meta)
	durationClient := time.Since(t0)
	fmt.Printf("Client,\thost key = %s\tduration = %s\n", hex.EncodeToString(clientKey.Key[:]), durationClient)

	// Server: get the Secret Value (SV) for the protocol and derive all subsequent keys in-process
	server := NewServer(ctx, sciondForServer)
	sv := server.fetchSV(ctx, drkey.SVMeta{
		Validity: meta.Validity,
		ProtoId:  meta.ProtoId,
	})
	t0 = time.Now()
	serverKey := server.HostHostKey(sv, meta)
	durationServer := time.Since(t0)

	fmt.Printf("Server,\thost key = %s\tduration = %s\n", hex.EncodeToString(serverKey.Key[:]), durationServer)
}

func getSecretFromReply(
	proto drkey.Protocol,
	rep *dkpb.SVResponse,
) (drkey.SV, error) {

	err := rep.EpochBegin.CheckValid()
	if err != nil {
		return drkey.SV{}, err
	}
	err = rep.EpochEnd.CheckValid()
	if err != nil {
		return drkey.SV{}, err
	}
	epoch := drkey.Epoch{
		Validity: cppki.Validity{
			NotBefore: rep.EpochBegin.AsTime(),
			NotAfter:  rep.EpochEnd.AsTime(),
		},
	}
	returningKey := drkey.SV{
		ProtoId: proto,
		Epoch:   epoch,
	}
	copy(returningKey.Key[:], rep.Key)
	return returningKey, nil
}
